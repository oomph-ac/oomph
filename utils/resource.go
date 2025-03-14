package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

// ResourcePacks loads all resource packs in a path.
func ResourcePacks(path string, contentKeyFile string) ([]*resource.Pack, error) {
	var contentKeys = make(map[string]string)
	if dat, err := os.ReadFile(path + "/" + contentKeyFile); err != nil {
		return nil, err
	} else {
		if err := json.Unmarshal(dat, &contentKeys); err != nil {
			return nil, err
		}
	}

	var packs []*resource.Pack
	if _, err := os.Stat(path); err == nil {
		if err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if strings.HasSuffix(info.Name(), ".zip") || strings.HasSuffix(info.Name(), ".mcpack") {
				pack, err := resource.ReadPath(path)
				if err != nil {
					return err
				}

				if key, ok := contentKeys[string(pack.UUID().String())]; ok {
					fmt.Println("applied", key, "to", pack.Name())
					pack = pack.WithContentKey(key)
				}
				packs = append(packs, pack)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	return packs, nil
}
