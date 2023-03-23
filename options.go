package funnel

import "time"

type Option func(*Config)

// WithTimeout defines the maximum time that goroutines will wait for ending of operation (the default is one minute)
func WithTimeout(t time.Duration) Option {
	return func(cfg *Config) {
		cfg.timeout = t
	}
}

// WithCacheTtl defines the time for which the result can remain cached (the default is 0 )
func WithCacheTtl(cTtl time.Duration) Option {
	return func(cfg *Config) {
		cfg.cacheTtl = cTtl
	}
}

// WithShouldCachePredicate allows more control over which responses should be cached.
// If this option is used responses from execute will only be cached if the predicate provided returns true.
func WithShouldCachePredicate(p func(interface{}, error) bool) Option {
	return func(cfg *Config) {
		cfg.shouldCache = p
	}
}
