package controllers

type IResource interface {
	Apply() (err error)
	UpdateStatus() error
	Type() string
}
