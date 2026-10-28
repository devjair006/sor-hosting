package database
import("context";
"path/filepath";
"testing")
func TestMigrationsReopenExistingDatabase(t *testing.T){p:=filepath.Join(t.TempDir(),"panel.db");
ctx:=context.Background();
db,e:=Open(ctx,p);
if e!=nil{t.Fatal(e)};
if e=Migrate(ctx,db);
e!=nil{t.Fatal(e)};
db.Close();
db,e=Open(ctx,p);
if e!=nil{t.Fatal(e)};
defer db.Close();
if e=Migrate(ctx,db);
e!=nil{t.Fatal(e)};
var n int;
if e=db.QueryRow("SELECT count(*) FROM servers").Scan(&n);
e!=nil||n!=1{t.Fatalf("servers=%d err=%v",n,e)}}
func TestForeignKeysEnforced(t *testing.T){db,e:=Open(context.Background(),filepath.Join(t.TempDir(),"test.db"));
if e!=nil{t.Fatal(e)};
defer db.Close();
if e=Migrate(context.Background(),db);
e!=nil{t.Fatal(e)};
_,e=db.Exec("INSERT INTO sites(id,server_id,name,domain,state,metadata,created_at,updated_at) VALUES('x','missing','x','a.example','active','{}','now','now')");
if e==nil{t.Fatal("foreign key accepted")}}
