package console

import (
 "net/http/httptest"
 "testing"
)

func TestLoginReferrerPolicyPreservesSameOriginPOST(t *testing.T) {
 s,_:=New("http://127.0.0.1:8080/api/v1",false)
 w:=httptest.NewRecorder();s.Handler().ServeHTTP(w,httptest.NewRequest("GET","/login",nil))
 // no-referrer makes Chromium send Origin: null for non-TLS form POSTs.
 // same-origin keeps our own POST verifiable while hiding cross-origin referrers.
 if got:=w.Header().Get("Referrer-Policy");got!="same-origin" {t.Fatalf("policy %q breaks browser origin validation on HTTP",got)}
}
