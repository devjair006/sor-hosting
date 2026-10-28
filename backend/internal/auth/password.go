package auth
import("crypto/rand";
"crypto/subtle";
"encoding/base64";
"fmt";
"strings";
"unicode/utf8";
"golang.org/x/crypto/argon2")
func HashPassword(password string)(string,error){if n:=utf8.RuneCountInString(password);
n<12||n>128{return "",fmt.Errorf("la contraseña debe contener entre 12 y 128 caracteres")};
salt:=make([]byte,16);
if _,e:=rand.Read(salt);
e!=nil{return "",e};
key:=argon2.IDKey([]byte(password),salt,3,64*1024,1,32);
enc:=base64.RawStdEncoding;
return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=1$%s$%s",enc.EncodeToString(salt),enc.EncodeToString(key)),nil}
func VerifyPassword(hash,password string)bool{if len(password)>512{return false};
p:=strings.Split(hash,"$");
if len(p)!=6||p[1]!="argon2id"||p[2]!="v=19"||p[3]!="m=65536,t=3,p=1"{return false};
s,e:=base64.RawStdEncoding.DecodeString(p[4]);
if e!=nil||len(s)!=16{return false};
want,e:=base64.RawStdEncoding.DecodeString(p[5]);
if e!=nil||len(want)!=32{return false};
key:=argon2.IDKey([]byte(password),s,3,64*1024,1,32);
return subtle.ConstantTimeCompare(key,want)==1}
