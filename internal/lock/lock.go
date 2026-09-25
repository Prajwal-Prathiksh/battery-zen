package lock

type Instance interface {
	Acquire() (bool, error)
	Release()
}

func New(path string) Instance {
	return newPlatformLock(path)
}
