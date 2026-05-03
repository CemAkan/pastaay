package config

import (
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

func WatchConfig(filePath string, reloadCallback func(*PastaayConfig)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	var timer *time.Timer
	var timerMu sync.Mutex
	var reattachTimer *time.Timer
	var reattachMu sync.Mutex

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Chmod) {
					reattachMu.Lock()
					if reattachTimer != nil {
						if !reattachTimer.Stop() {
							select {
							case <-reattachTimer.C:
							default:
							}
						}
					}
					reattachTimer = time.AfterFunc(100*time.Millisecond, func() {
						log.Println("Pastaay: Config file update detected. Re-attaching...")
						for i := 0; i < 5; i++ {
							time.Sleep(1 * time.Second)
							if err := watcher.Add(filePath); err == nil {
								if newCfg, loadErr := LoadConfig(filePath); loadErr == nil {
									reloadCallback(newCfg)
									return
								}
							}
						}
					})
					reattachMu.Unlock()
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					timerMu.Lock()
					if timer != nil {
						if !timer.Stop() {
							select {
							case <-timer.C:
							default:
							}
						}
					}
					timer = time.AfterFunc(500*time.Millisecond, func() {
						var newCfg *PastaayConfig
						var loadErr error
						for i := 0; i < 3; i++ {
							time.Sleep(100 * time.Millisecond * time.Duration(i+1))
							if newCfg, loadErr = LoadConfig(filePath); loadErr == nil {
								break
							}
						}
						if loadErr == nil {
							reloadCallback(newCfg)
						}
					})
					timerMu.Unlock()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Pastaay: Watcher error: %v\n", err)
			}
		}
	}()
	return watcher.Add(filePath)
}
