package configenv

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func Load(path string, target any, bindings ...Binding) error {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if err := Bind(v, bindings...); err != nil {
		return fmt.Errorf("bind environment: %w", err)
	}
	if err := v.Unmarshal(target); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	return nil
}
