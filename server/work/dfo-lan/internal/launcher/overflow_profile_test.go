package launcher

import (
	"encoding/json"
	"testing"
)

func TestOverflowMailProfileStrictFlag(t *testing.T) {
	for _, value := range []string{"0", "1", "yes", "2"} {
		p := Profile{Env: map[string]string{}}
		raw, _ := json.Marshal(value)
		err := p.applyEnvironment(t.TempDir(), "DFO_BOOST_AUTO_OVERFLOW_MAIL", raw)
		valid := value == "0" || value == "1"
		if (err == nil) != valid {
			t.Fatalf("flag %q: %v", value, err)
		}
		if valid && p.Env["DFO_BOOST_AUTO_OVERFLOW_MAIL"] != value {
			t.Fatal("flag was not forwarded")
		}
	}
}
