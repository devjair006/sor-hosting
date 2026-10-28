package wordpress
import("os";
"path/filepath";
"testing";
"golang.org/x/sys/unix")
func TestSecretPermissionsSurviveRestrictiveUmask(t *testing.T){old:=unix.Umask(0077);
defer unix.Umask(old);
path:=filepath.Join(t.TempDir(),"db-password");
if e:=writeSecret(path,"test",0444);
e!=nil{t.Fatal(e)};
st,e:=os.Stat(path);
if e!=nil||st.Mode().Perm()!=0444{t.Fatalf("PHP cannot read secret: %v %v",st,e)}}
