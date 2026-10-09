package whatsmeow_service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/EvolutionAPI/evolution-go/pkg/config"
)

// Valida a semântica do getAuthContainer compartilhado, usando o caminho SQLite
// (auto-contido, sem serviço externo — roda em qualquer CI):
//  1. falha na criação NÃO é memorizada (retry possível em vez de erro cacheado pra sempre)
//  2. sucesso é memorizado e o MESMO container é reusado nas chamadas seguintes
func TestGetAuthContainerRetryAndReuse(t *testing.T) {
	// Falha: exPath aponta pra diretório inexistente — o SQLite não cria
	// diretórios intermediários, então o Upgrade falha na primeira conexão.
	bad := whatsmeowService{
		config: &config.Config{},
		exPath: filepath.Join(t.TempDir(), "does-not-exist"),
	}
	bad.authStore = newAuthStore(context.Background(), nil, "", bad.exPath, "")
	t.Cleanup(func() {
		if err := bad.authStore.close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if _, err := bad.getAuthContainer(); err == nil {
		t.Fatal("esperava erro com diretório inexistente, veio nil")
	}
	if bad.authStore.container != nil {
		t.Fatal("failed store was published")
	}

	// Sucesso: exPath válido com o subdiretório dbdata esperado pelo DSN.
	goodPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(goodPath, "dbdata"), 0o755); err != nil {
		t.Fatal(err)
	}
	good := whatsmeowService{
		config: &config.Config{},
		exPath: goodPath,
	}
	good.authStore = newAuthStore(context.Background(), nil, "", good.exPath, "")
	t.Cleanup(func() {
		if err := good.authStore.close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	c1, err := good.getAuthContainer()
	if err != nil {
		t.Fatalf("retry após falha deveria funcionar em vez de devolver erro cacheado: %v", err)
	}
	if c1 == nil {
		t.Fatal("container nil após sucesso")
	}

	c2, err := good.getAuthContainer()
	if err != nil {
		t.Fatalf("segunda chamada falhou: %v", err)
	}
	if c1 != c2 {
		t.Fatal("container não foi reusado — deveria ser singleton compartilhado")
	}
}
