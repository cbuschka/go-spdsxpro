package internal

import "fmt"

type stringMapping map[string]int

func (s stringMapping) toInt(strVal string) (int, error) {
	intVal, found := s[strVal]
	if !found {
		return 0, fmt.Errorf("invalid value %s", strVal)
	}

	return intVal, nil
}

func (s stringMapping) toString(intVal int) (string, error) {
	for key, val := range s {
		if val == intVal {
			return key, nil
		}
	}

	return "", fmt.Errorf("invalid value %d", intVal)
}
