package configkeys

const (
	delimiter = "."

	ConfigPrefix = "config"

	ConfigEffectPrefix = ConfigPrefix + delimiter + "effect"

	ConfigEffectStatePrefix = ConfigEffectPrefix + delimiter + "state"

	ConfigEffectStateHandlerPrefix     = ConfigEffectStatePrefix + delimiter + "handler"
	ConfigEffectStateHandlerBufferSize = ConfigEffectStateHandlerPrefix + delimiter + "buffer_size"
	ConfigEffectStateHandlerNumWorkers = ConfigEffectStateHandlerPrefix + delimiter + "num_workers"

	ConfigEffectBindingPrefix = ConfigEffectPrefix + delimiter + "binding"

	ConfigEffectBindingHandlerPrefix     = ConfigEffectBindingPrefix + delimiter + "handler"
	ConfigEffectBindingHandlerBufferSize = ConfigEffectBindingHandlerPrefix + delimiter + "buffer_size"
	ConfigEffectBindingHandlerNumWorkers = ConfigEffectBindingHandlerPrefix + delimiter + "num_workers"

	ConfigEffectLogPrefix = ConfigEffectPrefix + delimiter + "log"

	ConfigEffectLogHandlerPrefix     = ConfigEffectLogPrefix + delimiter + "handler"
	ConfigEffectLogHandlerBufferSize = ConfigEffectLogHandlerPrefix + delimiter + "buffer_size"
	ConfigEffectLogHandlerNumWorkers = ConfigEffectLogHandlerPrefix + delimiter + "num_workers"

	ConfigEffectConcurrencyPrefix = ConfigEffectPrefix + delimiter + "concurrency"

	ConfigEffectConcurrencyHandlerPrefix     = ConfigEffectConcurrencyPrefix + delimiter + "handler"
	ConfigEffectConcurrencyHandlerBufferSize = ConfigEffectConcurrencyHandlerPrefix + delimiter + "buffer_size"
	ConfigEffectConcurrencyHandlerNumWorkers = ConfigEffectConcurrencyHandlerPrefix + delimiter + "num_workers"
)
