package document

import "bytes"

type localeKeys interface {
	key(string) ([]byte, error)
}

func compareLocaleKeys(left, right []byte) int {
	if left == nil && right != nil {
		return -1
	}
	if left != nil && right == nil {
		return 1
	}
	return bytes.Compare(left, right)
}
