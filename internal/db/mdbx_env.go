package db

import (
	"errors"

	"github.com/torquem-ch/mdbx-go/mdbx"
)

var (
	env         *mdbx.Env
	initialized bool
)

var ErrEnvNoInitialized = errors.New("environment is no initialize")

// InitializeMDBXEnv initializes the Juno LMDB environment.
func InitializeMDBXEnv(path string, optMaxDB uint64, flags uint) (err error) {
	defer func() {
		if err == nil {
			initialized = true
		}
	}()

	env, err = NewMDBXEnv(path, optMaxDB, flags)
	return err
}

func NewMDBXEnv(path string, optMaxDB uint64, flags uint) (*mdbx.Env, error) {
	env, err := mdbx.NewEnv()
	if err != nil {
		// notest
		return nil, err
	}
	err = env.SetOption(mdbx.OptMaxDB, optMaxDB)
	if err != nil {
		// notest
		return nil, err
	}
	err = env.SetGeometry(268435456, 268435456, 25769803776, 268435456, 268435456, 4096)
	if err != nil {
		// notest
		return nil, err
	}
	err = env.Open(path, flags|mdbx.Exclusive, 0o664)
	if err != nil {
		// notest
		return nil, err
	}
	return env, nil
}

// GetMDBXEnv returns the Juno LMDB environment. If the environment is not
// initialized then ErrEnvNoInitialized is returned.
func GetMDBXEnv() (*mdbx.Env, error) {
	if !initialized {
		return nil, ErrEnvNoInitialized
	}
	return env, nil
}
