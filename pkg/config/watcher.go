package config

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceWindow = 300 * time.Millisecond

// WatchConfig watches filePath and invokes reloadCallback on each successful reload.
func WatchConfig(filePath string, reloadCallback func(*PastaayConfig)) (cancel func(), err error) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	watcher, werr := fsnotify.NewWatcher()
	if werr != nil {
		cancelCtx()
		return nil, werr
	}
	if aerr := watcher.Add(filePath); aerr != nil {
		_ = watcher.Close()
		cancelCtx()
		return nil, aerr
	}

	var (
		timer   *time.Timer
		timerMu sync.Mutex
	)

	// scheduleReload coalesces every kind of event onto the debounce window.
	scheduleReload := func(reason string) {
		timerMu.Lock()
		defer timerMu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(debounceWindow, func() {

			_ = watcher.Remove(filePath)
			for attempt := 0; attempt < 10; attempt++ {
				if err := watcher.Add(filePath); err == nil {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}

			newCfg, loadErr := LoadConfig(filePath)
			if loadErr != nil {
				log.Printf("[Pastaay-Config] reload skipped (%s): %v", reason, loadErr)
				return
			}
			if vErr := newCfg.Validate(); vErr != nil {
				log.Printf("[Pastaay-Config] reload rejected (%s): %v", reason, vErr)
				return
			}
			reloadCallback(newCfg)
		})
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) ||
					event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) ||
					event.Has(fsnotify.Chmod) {
					scheduleReload(event.Op.String())
				}
			case errEv, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[Pastaay-Config] watcher error (forcing reattach): %v", errEv)
				scheduleReload("watcher-error")
			}
		}
	}()

	cancel = func() {
		cancelCtx()
		// Stop any pending debounce timer to avoid spurious post-cancel reloads.
		timerMu.Lock()
		if timer != nil {
			timer.Stop()
		}
		timerMu.Unlock()
		wg.Wait()
	}
	return cancel, nil
}
