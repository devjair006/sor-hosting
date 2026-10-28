package config
import "testing"
func TestLoadRejectsMissingProductionSecrets(t *testing.T) { t.Setenv("SOR_MODE", "production");
 t.Setenv("SOR_SECRET_KEY", "");
 if _,err:=Load();
err==nil {t.Fatal("production accepted missing encryption key")} }
