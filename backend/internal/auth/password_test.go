package auth
import "testing"
func TestArgon2RoundTripAndMalformedPHC(t *testing.T){h,e:=HashPassword("correct horse battery");
if e!=nil{t.Fatal(e)};
if !VerifyPassword(h,"correct horse battery")||VerifyPassword(h,"wrong"){t.Fatal("password verification broken")};
for _,h:=range []string{"", "$argon2id$v=19$m=999999999,t=9,p=100$AAAA$AAAA"}{if VerifyPassword(h,"x"){t.Fatal("invalid PHC accepted")}}}
