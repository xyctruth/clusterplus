package controllers

type ReconcileConfig struct {
	Enable  bool            `yaml:"enable"`
	Details map[string]bool `yaml:"details"`
}
