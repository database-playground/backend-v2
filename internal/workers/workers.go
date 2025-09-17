package workers

import (
	"sync"
)

var Global = &sync.WaitGroup{}
