package config

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

func WatchConfig(filePath string, reloadCallback func(*PastaayConfig)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	var (
		timer         *time.Timer
		timerMu       sync.Mutex
		isReattaching atomic.Bool
	)

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Chmod) {
					if isReattaching.CompareAndSwap(false, true) {
						go func(op fsnotify.Op) {
							defer isReattaching.Store(false)
							log.Printf("Pastaay: Config inode changed [%s]. Attempting re-attach...", op)
							for i := 0; i < 10; i++ {
								time.Sleep(500 * time.Millisecond)
								if err := watcher.Add(filePath); err == nil {
									if newCfg, loadErr := LoadConfig(filePath); loadErr == nil {
										reloadCallback(newCfg)
										log.Println("Pastaay: Watcher re-attached successfully.")
										return
									}
								}
							}
							log.Printf("Pastaay: Failed to re-attach watcher to %s after 10 attempts.", filePath)
						}(event.Op)
					}
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					timerMu.Lock()
					if timer != nil {
						timer.Stop()
					}
					timer = time.AfterFunc(300*time.Millisecond, func() {
						newCfg, loadErr := LoadConfig(filePath)
						if loadErr != nil {
							log.Printf("Pastaay: Hot-reload skipped. Invalid configuration: %v", loadErr)
							return
						}
						reloadCallback(newCfg)
					})
					timerMu.Unlock()
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Pastaay: Watcher runtime error: %v", err)
			}
		}
	}()
	return watcher.Add(filePath)
}
