package plugins

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const DefaultDiscoveryDebounce = 250 * time.Millisecond

type DiscoveredPlugin struct {
	Manifest       Manifest
	Dir            string
	ManifestPath   string
	ExecutablePath string
}

type DiscoveryWatcherConfig struct {
	PluginsDir string
	Debounce   time.Duration
	OnChange   func([]DiscoveredPlugin)
	OnError    func(error)
	Ready      chan<- struct{}
}

func ScanPluginsDir(pluginsDir string) ([]DiscoveredPlugin, error) {
	pluginsDir = strings.TrimSpace(pluginsDir)
	if pluginsDir == "" {
		return nil, errors.New("plugins directory is required")
	}

	root, err := filepath.Abs(pluginsDir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("plugins path is not a directory: %s", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	discovered := make([]DiscoveredPlugin, 0, len(entries))
	var scanErrors []error
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(root, entry.Name())
		pluginDir, err = filepath.Abs(pluginDir)
		if err != nil {
			scanErrors = append(scanErrors, err)
			continue
		}
		if !pathInside(root, pluginDir) {
			scanErrors = append(scanErrors, fmt.Errorf("plugin directory escapes root: %s", pluginDir))
			continue
		}

		manifestPath := filepath.Join(pluginDir, ManifestFileName)
		if _, err := os.Stat(manifestPath); err != nil {
			if !os.IsNotExist(err) {
				scanErrors = append(scanErrors, fmt.Errorf("%s: %w", manifestPath, err))
			}
			continue
		}

		manifest, err := LoadManifest(pluginDir)
		if err != nil {
			scanErrors = append(scanErrors, fmt.Errorf("%s: %w", manifestPath, err))
			continue
		}
		executablePath, err := manifest.ExecutablePath(pluginDir)
		if err != nil {
			scanErrors = append(scanErrors, fmt.Errorf("%s: %w", manifestPath, err))
			continue
		}

		discovered = append(discovered, DiscoveredPlugin{
			Manifest:       *manifest,
			Dir:            pluginDir,
			ManifestPath:   manifestPath,
			ExecutablePath: executablePath,
		})
	}

	sort.Slice(discovered, func(i, j int) bool {
		return discovered[i].Manifest.ID < discovered[j].Manifest.ID
	})

	return discovered, errors.Join(scanErrors...)
}

func WatchPluginsDir(ctx context.Context, cfg DiscoveryWatcherConfig) error {
	if ctx == nil {
		ctx = context.Background()
	}
	pluginsDir := strings.TrimSpace(cfg.PluginsDir)
	if pluginsDir == "" {
		return errors.New("plugins directory is required")
	}
	root, err := filepath.Abs(pluginsDir)
	if err != nil {
		return err
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = DefaultDiscoveryDebounce
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	watches := map[string]struct{}{}
	if err := syncPluginWatches(root, watcher, watches); err != nil {
		return err
	}
	signalWatcherReady(cfg.Ready)

	var timer *time.Timer
	var timerC <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	schedule := func() {
		if timer == nil {
			timer = time.NewTimer(cfg.Debounce)
			timerC = timer.C
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(cfg.Debounce)
		timerC = timer.C
	}

	emitScan := func() {
		if err := syncPluginWatches(root, watcher, watches); err != nil && cfg.OnError != nil {
			cfg.OnError(err)
		}
		discovered, err := ScanPluginsDir(root)
		if err != nil && cfg.OnError != nil {
			cfg.OnError(err)
		}
		if cfg.OnChange != nil {
			cfg.OnChange(discovered)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if isPluginDiscoveryEvent(root, event) {
				schedule()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			if cfg.OnError != nil {
				cfg.OnError(err)
			}
		case <-timerC:
			timerC = nil
			timer = nil
			emitScan()
		}
	}
}

func syncPluginWatches(root string, watcher *fsnotify.Watcher, watches map[string]struct{}) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	desired := map[string]struct{}{filepath.Clean(root): {}}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(root, entry.Name())
		pluginDir, err = filepath.Abs(pluginDir)
		if err != nil || !pathInside(root, pluginDir) {
			continue
		}
		desired[filepath.Clean(pluginDir)] = struct{}{}
	}

	var result error
	for watched := range watches {
		if _, ok := desired[watched]; ok {
			continue
		}
		if err := watcher.Remove(watched); err != nil {
			result = errors.Join(result, err)
		}
		delete(watches, watched)
	}
	for path := range desired {
		if _, ok := watches[path]; ok {
			continue
		}
		if err := watcher.Add(path); err != nil {
			result = errors.Join(result, err)
			continue
		}
		watches[path] = struct{}{}
	}

	return result
}

func isPluginDiscoveryEvent(root string, event fsnotify.Event) bool {
	if event.Name == "" || event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}
	name := filepath.Clean(event.Name)
	if name == filepath.Clean(root) || filepath.Base(name) == ManifestFileName {
		return true
	}
	return pathInside(root, name)
}

func signalWatcherReady(ready chan<- struct{}) {
	if ready == nil {
		return
	}
	select {
	case ready <- struct{}{}:
	default:
	}
}
