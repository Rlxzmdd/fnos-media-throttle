package downloader

import (
	"context"
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (g *fnosGateway) login(ctx context.Context, downloader Downloader) error {
	username := strings.TrimSpace(downloader.Username)
	password := downloader.Password
	if username == "" || password == "" {
		return errors.New("飞牛登录用户名和密码不能为空")
	}
	publicResponse, err := g.request(ctx, "util.crypto.getRSAPub", nil)
	if err != nil {
		return err
	}
	publicData := mergeFNOSMaps(publicResponse, unwrapFNOSMap(publicResponse))
	publicPEM, _ := publicData["pub"].(string)
	si := fmt.Sprint(publicData["si"])
	if publicPEM == "" || si == "<nil>" {
		return errors.New("飞牛网关未返回 RSA 公钥或 si")
	}
	debugf(ctx, "飞牛认证已取得公钥：si存在=%t，RSA位数=%d", si != "", publicKeyBits(publicPEM))
	aesKeyText, err := randomAlphaNumeric(32)
	if err != nil {
		return err
	}
	aesKey := []byte(aesKeyText)
	iv := make([]byte, aes.BlockSize)
	if _, err = rand.Read(iv); err != nil {
		return err
	}
	loginID := g.requestID()
	deviceID := fnosDeviceID(downloader)
	loginJSON, err := json.Marshal(fnosLoginPayload(username, password, si, deviceID, loginID))
	if err != nil {
		return err
	}
	debugf(ctx, "飞牛认证准备登录：用户名长度=%d，reqid=%s，deviceName=Linux-Google Chrome，deviceID存在=%t，stay=true", len([]rune(username)), loginID, deviceID != "")
	encryptedLogin, err := aesCBCEncrypt(aesKey, iv, loginJSON)
	if err != nil {
		return err
	}
	publicKey, err := parseRSAPublicKey(publicPEM)
	if err != nil {
		return err
	}
	encryptedKey, err := rsa.EncryptPKCS1v15(rand.Reader, publicKey, aesKey)
	if err != nil {
		return err
	}
	wrapper, _ := json.Marshal(map[string]any{"req": "encrypted", "iv": base64.StdEncoding.EncodeToString(iv), "rsa": base64.StdEncoding.EncodeToString(encryptedKey), "aes": base64.StdEncoding.EncodeToString(encryptedLogin)})
	if deadline, ok := ctx.Deadline(); ok {
		_ = g.conn.SetDeadline(deadline)
	} else {
		_ = g.conn.SetDeadline(time.Now().Add(15 * time.Second))
	}
	if err = g.writeFrame(1, wrapper); err != nil {
		return err
	}
	loginResponse, err := g.readResponse(ctx, loginID, "user.login")
	if err != nil {
		return fmt.Errorf("飞牛登录失败: %w", err)
	}
	loginData := mergeFNOSMaps(loginResponse, unwrapFNOSMap(loginResponse))
	if result, _ := loginData["result"].(string); result != "" && result != "succ" {
		return fmt.Errorf("飞牛登录失败: %v", loginData["errmsg"])
	}
	secretText, _ := loginData["secret"].(string)
	if secretText == "" && stringValue(loginData["accessToken"]) != "" {
		if boolValue(loginData["isBindTwofaSecret"]) && !boolValue(loginData["isTrustedDevice"]) {
			return errors.New("飞牛账号启用了两步验证，需要验证码才能登录；请先将此设备设为受信任设备")
		}
		if boolValue(loginData["isTwofaEnforced"]) && !boolValue(loginData["isBindTwofaSecret"]) {
			return errors.New("飞牛账号要求设置两步验证，请先在系统中完成两步验证绑定")
		}
	}
	secret, decodeErr := base64.StdEncoding.DecodeString(secretText)
	if decodeErr != nil || len(secret) == 0 {
		return errors.New("飞牛登录未返回有效签名密钥")
	}
	plainSecret, err := aesCBCDecrypt(aesKey, iv, secret)
	if err != nil {
		return fmt.Errorf("解密飞牛签名密钥: %w", err)
	}
	g.signKey = plainSecret
	if backID, _ := loginData["backId"].(string); backID != "" {
		g.backID = backID
	}
	g.index = 1
	configuredUID := g.uid
	loginUID := int64Value(loginData["uid"])
	uid, uidSource := fnosBusinessUID(loginUID, configuredUID)
	g.uid = uid
	if uidSource == "configured" {
		debugf(ctx, "飞牛登录响应未包含 uid，回退使用界面配置的用户 ID=%d", configuredUID)
	} else if uidSource == "default" {
		debugf(ctx, "飞牛登录响应未包含 uid，回退使用默认用户 ID=1000")
	}
	debugf(ctx, "飞牛认证成功：响应uid=%d，配置uid=%d，实际业务uid=%d，uid来源=%s，backId长度=%d，签名密钥指纹=%s", loginUID, configuredUID, g.uid, uidSource, len(g.backID), shortKeyFingerprint(g.signKey))
	return nil
}

func fnosBusinessUID(loginUID, configuredUID int64) (int64, string) {
	if loginUID > 0 {
		return loginUID, "login-response"
	}
	if configuredUID > 0 {
		return configuredUID, "configured"
	}
	return 1000, "default"
}

func (g *fnosGateway) transferConfig(ctx context.Context) (map[string]any, error) {
	response, err := g.request(ctx, "appcgi.downloadcenter.config.getTransferCfg", map[string]any{"uid": g.uid})
	if err != nil {
		return nil, err
	}
	config := unwrapFNOSMap(response)
	if nested, ok := config["transfer_cfg"].(map[string]any); ok {
		config = nested
	}
	if len(config) == 0 {
		return nil, errors.New("飞牛下载未返回传输配置")
	}
	return config, nil
}
