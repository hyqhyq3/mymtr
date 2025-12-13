package mtr

import (
	"errors"
	"net"
	"strings"
)

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func looksLikePermission(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "operation not permitted") || strings.Contains(s, "permission denied")
}
