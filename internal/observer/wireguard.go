package observer

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// ReadWGHandshake возвращает возраст последнего handshake в секундах для интерфейса wg.
// Формат вывода: "latest handshake: 42 seconds ago" или "latest handshake: 3 minutes ago".
// Если handshake не было или интерфейс недоступен, возвращает -1.
func ReadWGHandshake(iface string) (ageSec int, err error) {
	out, err := exec.Command("wg", "show", iface).Output()
	if err != nil {
		return -1, err
	}
	// "latest handshake: 123 seconds ago" или "latest handshake: 2 minutes ago"
	reSec := regexp.MustCompile(`latest handshake:\s*(\d+)\s*seconds?\s*ago`)
	reMin := regexp.MustCompile(`latest handshake:\s*(\d+)\s*minutes?\s*ago`)
	line := strings.TrimSpace(string(out))
	for _, l := range strings.Split(line, "\n") {
		l = strings.TrimSpace(l)
		if m := reSec.FindStringSubmatch(l); len(m) == 2 {
			n, _ := strconv.Atoi(m[1])
			return n, nil
		}
		if m := reMin.FindStringSubmatch(l); len(m) == 2 {
			n, _ := strconv.Atoi(m[1])
			return n * 60, nil
		}
	}
	return -1, nil
}
