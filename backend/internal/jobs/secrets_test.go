package jobs
import("bytes";
"testing";
"sor.hosting/shared/secretbox")
func TestSecretsEncryptedAndAuthenticated(t *testing.T){key:=bytes.Repeat([]byte{7},32);
v,e:=secretbox.Seal(key,[]byte("sensitive-password"));
if e!=nil{t.Fatal(e)};
if bytes.Contains([]byte(v),[]byte("sensitive")){t.Fatal("plaintext persisted")};
b,e:=secretbox.Open(key,v);
if e!=nil||string(b)!="sensitive-password"{t.Fatal("roundtrip")};
if _,e=secretbox.Open(bytes.Repeat([]byte{8},32),v);
e==nil{t.Fatal("wrong key accepted")}}
