package console

import "testing"

func TestDashboardUptimeFormatting(t *testing.T) {
	for _, key := range []string{"uptime_seconds", "server_uptime_seconds"} {
		for _, tc := range []struct {
			seconds any
			want    string
		}{{float64(90061), "1:01:01"}, {float64(59), "0:00:00"}, {float64(3600), "0:01:00"}, {nil, "—"}} {
			if got := value(Record{key: tc.seconds}, key); got != tc.want {
				t.Fatalf("%s %v: got %s want %s", key, tc.seconds, got, tc.want)
			}
		}
	}
	if human("uptime_seconds") != "Uptime GoVPN Service (day:hour:menit)" {
		t.Fatal("service label")
	}
	if human("server_uptime_seconds") != "Server Uptime (day:hour:menit)" {
		t.Fatal("server label")
	}
}
