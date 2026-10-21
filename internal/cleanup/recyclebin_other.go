//go:build !windows

package cleanup

import "errors"

func queryRecycleBin() (uint64, int64, error) { return 0, 0, errors.New("unsupported") }
func emptyRecycleBin() (uint64, int64, error) { return 0, 0, errors.New("unsupported") }
