package service

import "context"

func (e *Engine) PollLibrary(ctx context.Context, library Library) {
	_ = e.Coordinate(ctx, func(ctx context.Context) error {
		current, err := e.Store.Library(ctx, library.ID)
		if err != nil {
			return err
		}
		if current.Enabled {
			e.pollLibrary(ctx, current)
		}
		return nil
	})
}

func (e *Engine) pollLibrary(ctx context.Context, library Library) {
	ctx = e.DebugContext(ctx)
	debugf(ctx, "开始轮询影视库：名称=%s，类型=%s，轮询=%d 秒", library.Name, library.Kind, library.PollSeconds)
	count, err := e.Media.ActiveViewerCount(ctx, library)
	if err != nil {
		_ = e.Store.SetLibraryStatus(ctx, library.ID, library.LastCount, err.Error())
		return
	}
	_ = e.Store.SetLibraryStatus(ctx, library.ID, count, "")
	debugf(ctx, "影视库采集结果：名称=%s，类型=%s，活跃观看=%d", library.Name, library.Kind, count)
}
