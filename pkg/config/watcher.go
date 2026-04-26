package config

import (
	"log"

	"github.com/fsnotify/fsnotify"
)

// WatchConfig monitors the configuration file for changes in real-time.
// When a change is detected, it triggers the reloadCallback with the new configuration.
func WatchConfig(filePath string, reloadCallback func(*PastaayConfig)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Start a concurrent goroutine to listen for file events
	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Check if the file was modified or saved
				if event.Has(fsnotify.Write) {
					log.Println("Pastaay: Configuration file changed, reloading...")

					newCfg, err := LoadConfig(filePath)
					if err != nil {
						log.Printf("Pastaay: Error reloading config: %v\n", err)
						continue
					}

					// Send the new configuration to the application safely
					reloadCallback(newCfg)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Pastaay: Watcher error: %v\n", err)
			}
		}
	}()

	// Tell the watcher which file to monitor
	return watcher.Add(filePath)
}
