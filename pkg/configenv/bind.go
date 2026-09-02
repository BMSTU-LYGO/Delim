package configenv

import "github.com/spf13/viper"

type Binding struct {
	Key string
	Env string
}

func Bind(v *viper.Viper, bindings ...Binding) error {
	for _, binding := range bindings {
		if err := v.BindEnv(binding.Key, binding.Env); err != nil {
			return err
		}
	}

	return nil
}
