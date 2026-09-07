package api

import (
	"errors"
	"net/http"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

func (a Handler) chains(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		chains, err := a.store.Chains(r.Context())
		if err != nil {
			fail(w, 500, err)
			return
		}
		jsonOut(w, 200, chains)
	case http.MethodPost:
		var chain domain.BindingChain
		if err := decode(r, &chain); err != nil {
			fail(w, 400, err)
			return
		}
		if err := a.store.SaveChain(r.Context(), &chain); err != nil {
			fail(w, 400, err)
			return
		}
		jsonOut(w, 201, chain)
	default:
		fail(w, 405, errors.New("method not allowed"))
	}
}

func (a Handler) chain(w http.ResponseWriter, r *http.Request) {
	id, action, err := pathID(r.URL.Path, "/api/v1/chains/")
	if err != nil {
		fail(w, 400, err)
		return
	}
	current, err := a.store.Chain(r.Context(), id)
	if err != nil {
		fail(w, 404, err)
		return
	}
	if action == "preview" && r.Method == http.MethodPost {
		libraries, readErr := a.store.LibrariesForChain(r.Context(), id)
		if readErr != nil {
			fail(w, 500, readErr)
			return
		}
		viewers := 0
		for _, library := range libraries {
			if library.Enabled {
				viewers += library.LastCount
			}
		}
		jsonOut(w, 200, map[string]any{"viewerCount": viewers, "uploadKB": current.Rule.Upload.Limit(viewers), "downloadKB": current.Rule.Download.Limit(viewers)})
		return
	}
	switch r.Method {
	case http.MethodGet:
		jsonOut(w, 200, current)
	case http.MethodPut:
		var updated domain.BindingChain
		if err := decode(r, &updated); err != nil {
			fail(w, 400, err)
			return
		}
		updated.ID = id
		if err := a.store.SaveChain(r.Context(), &updated); err != nil {
			fail(w, 400, err)
			return
		}
		for _, downloaderID := range current.DownloaderIDs {
			stillMember := false
			for _, updatedID := range updated.DownloaderIDs {
				if updatedID == downloaderID {
					stillMember = true
					break
				}
			}
			if (!updated.Enabled || !stillMember) && current.Enabled {
				if downloader, loadErr := a.store.Downloader(r.Context(), downloaderID); loadErr == nil {
					if restoreErr := a.engine.ReleaseIfUnbound(r.Context(), downloader); restoreErr != nil {
						a.store.Event(r.Context(), &downloaderID, "error", "绑定链变更后按退出策略释放限速失败: "+restoreErr.Error())
					}
				}
			} else if current.Enabled && updated.Enabled && stillMember {
				restoreUpload := current.Rule.Upload.Enabled && !updated.Rule.Upload.Enabled
				restoreDownload := current.Rule.Download.Enabled && !updated.Rule.Download.Enabled
				if restoreUpload || restoreDownload {
					if downloader, loadErr := a.store.Downloader(r.Context(), downloaderID); loadErr == nil {
						if restoreErr := a.engine.ReleaseDirections(r.Context(), downloader, restoreUpload, restoreDownload); restoreErr != nil {
							a.store.Event(r.Context(), &downloaderID, "error", "绑定链停止接管方向后按退出策略释放限速失败: "+restoreErr.Error())
						}
					}
				}
			}
		}
		jsonOut(w, 200, updated)
	case http.MethodDelete:
		downloaderIDs, deleteErr := a.store.DeleteChain(r.Context(), id)
		if deleteErr != nil {
			fail(w, 500, deleteErr)
			return
		}
		for _, downloaderID := range downloaderIDs {
			if downloader, loadErr := a.store.Downloader(r.Context(), downloaderID); loadErr == nil {
				if restoreErr := a.engine.ReleaseIfUnbound(r.Context(), downloader); restoreErr != nil {
					a.store.Event(r.Context(), &downloaderID, "error", "删除绑定链后按退出策略释放限速失败: "+restoreErr.Error())
				}
			}
		}
		jsonOut(w, http.StatusNoContent, nil)
	default:
		fail(w, 405, errors.New("method not allowed"))
	}
}
