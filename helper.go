package ini

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/henrylee2cn/goutil"
	"github.com/henrylee2cn/goutil/errors"
)

// SyncINI quickly create your own configuration files.
// Struct tags reference `https://github.com/henrylee2cn/ini`
func SyncINI(structPtr interface{}, f func(onecUpdateFunc func() error) error, filename ...string) error {
	t := reflect.TypeOf(structPtr)
	if t.Kind() != reflect.Ptr {
		return errors.New("SyncINI's param must be struct pointer type.")
	}
	t = t.Elem()
	if t.Kind() != reflect.Struct {
		return errors.New("SyncINI's param must be struct pointer type.")
	}

	var fname string
	if len(filename) > 0 {
		fname = filename[0]
	} else {
		fname = strings.TrimSuffix(t.Name(), "Config")
		fname = strings.TrimSuffix(fname, "INI")
		fname = goutil.SnakeString(fname) + ".ini"
	}
	var cfg *File
	var err error
	var existed bool
	cfg, err = Load(fname)
	if err != nil {
		os.MkdirAll(filepath.Dir(fname), 0777)
		cfg, err = LooseLoad(fname)
		if err != nil {
			return err
		}
	} else {
		existed = true
	}

	err = cfg.MapTo(structPtr)
	if err != nil {
		return err
	}

	var once sync.Once
	var onecUpdateFunc = func() error {
		var err error
		once.Do(func() {
			err = cfg.ReflectFrom(structPtr)
			if err != nil {
				return
			}
			err = cfg.SaveTo(fname)
			if err != nil {
				return
			}
		})
		return err
	}

	if f != nil {
		if err = f(onecUpdateFunc); err != nil {
			return err
		}
	}

	if !existed {
		return onecUpdateFunc()
	}
	return nil
}
