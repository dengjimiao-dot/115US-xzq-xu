package catalog

// R7 native COS lexer, isolated from the target's unchanged Bakal reader.
// Unknown display/reference values remain type 255 and cannot authorize gameplay.
import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strconv"
	"strings"
)

func readRaidCOSTokens(a *pvf.Archive, path string) ([]pvf.Token, error) {
	s, err := a.ReadText(path)
	if err != nil {
		return nil, err
	}
	var out []pvf.Token
	s = strings.Trim(s, "\ufeff\x00 \t\r\n")
	for len(s) > 0 {
		s = strings.TrimLeft(s, " \t\r\n")
		if s == "" {
			break
		}
		if strings.HasPrefix(s, "//") {
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
				continue
			}
			break
		}
		if s[0] == '[' || s[0] == '`' {
			end := byte(']')
			typ := byte(3)
			if s[0] == '`' {
				end = '`'
				typ = 6
			}
			i := strings.IndexByte(s[1:], end)
			if i < 0 {
				return nil, fmt.Errorf("%s: unclosed COS token", path)
			}
			i++
			value := s[:i+1]
			if typ == 6 {
				value = s[1:i]
			}
			out = append(out, pvf.Token{Type: typ, Text: value})
			s = s[i+1:]
			continue
		}
		i := strings.IndexAny(s, " \t\r\n[")
		if i < 0 {
			i = len(s)
		}
		if i == 0 {
			return nil, fmt.Errorf("%s: invalid COS token", path)
		}
		word := s[:i]
		s = s[i:]
		if v, e := strconv.ParseInt(word, 10, 32); e == nil {
			out = append(out, pvf.Token{Type: 0, Value: int32(v)})
		} else if v, e := strconv.ParseFloat(word, 32); e == nil {
			out = append(out, pvf.Token{Type: 2, Number: float32(v)})
		} else {
			out = append(out, pvf.Token{Type: 255, Text: word})
		}
	}
	return out, nil
}
