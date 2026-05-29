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
		timer    *time.Timer
		timerMu  sync.Mutex
		fireWG   sync.WaitGroup // tracks in-flight debounced reload bodies
		stopFlag = make(chan struct{})
	)

	scheduleReload := func(reason string) {
		timerMu.Lock()
		defer timerMu.Unlock()
		select {
		case <-stopFlag:
			return
		default:
		}
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(debounceWindow, func() {
			fireWG.Add(1)
			defer fireWG.Done()
			// Re-check stop flag
			select {
			case <-stopFlag:
				return
			default:
			}

			_ = watcher.Remove(filePath)
			for attempt := 0; attempt < 10; attempt++ {
				if err := watcher.Add(filePath); err == nil {
					break
				}
				select {
				case <-stopFlag:
					return
				case <-time.After(50 * time.Millisecond):
				}
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
			select {
			case <-stopFlag:
				return
			default:
			}
			reloadCallback(newCfg)
		})
	}

	var loopWG sync.WaitGroup
	loopWG.Add(1)
	go func() {
		defer loopWG.Done()
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
		close(stopFlag)
		cancelCtx()
		timerMu.Lock()
		if timer != nil {
			timer.Stop()
		}
		timerMu.Unlock()
		loopWG.Wait()
		fireWG.Wait()
	}
	return cancel, nil
}
