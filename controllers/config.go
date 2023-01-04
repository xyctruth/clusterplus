package controllers

type Config struct {
	ReconcileFilter Reconcile `yaml:"reconcile"`
}
type Reconcile struct {
	Enable  bool            `yaml:"enable"`
	Details map[string]bool `yaml:"details"`
}
