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

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					log.Println("Pastaay: Config file removed/renamed. Attempting to re-attach watcher...")

					go func(path string) {
						for i := 0; i < 5; i++ {
							time.Sleep(1 * time.Second)
							if err := watcher.Add(path); err == nil {
								log.Println("Pastaay: Watcher successfully re-attached.")

								if newCfg, loadErr := LoadConfig(path); loadErr == nil {
									log.Println("Pastaay: Config force-reloaded after file replacement.")
									reloadCallback(newCfg)
								}
								return
							}
						}
						log.Printf("[ERROR] Pastaay: CRITICAL - Failed to re-attach watcher to %s after 5 retries. Hot-reload is dead.\n", path)
					}(filePath)
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					timerMu.Lock()
					if timer != nil {
						timer.Stop()
					}

					timer = time.AfterFunc(500*time.Millisecond, func() {
						log.Println("Pastaay: Configuration change detected and debounced...")

						var newCfg *PastaayConfig
						var loadErr error

						for i := 0; i < 3; i++ {
							time.Sleep(100 * time.Millisecond * time.Duration(i+1))
							newCfg, loadErr = LoadConfig(filePath)
							if loadErr == nil {
								break
							}
						}

						if loadErr != nil {
							log.Printf("Pastaay: Error reloading config after retries: %v\n", loadErr)
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
				log.Printf("Pastaay: Watcher error: %v\n", err)
			}
		}
	}()

	return watcher.Add(filePath)
}
