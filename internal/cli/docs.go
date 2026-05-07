package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Сгенерировать Markdown-справку по всем CLI-командам",
	Long: `Генерирует Markdown-документацию по всем подкомандам jtpost через cobra/doc.
По умолчанию пишет в ./docs/cli/. Перезаписывает существующие файлы.`,
	RunE: runDocs,
}

func init() {
	docsCmd.Flags().StringP("out", "o", "./docs/cli", "директория для сгенерированных .md файлов")
	rootCmd.AddCommand(docsCmd)
}

func runDocs(cmd *cobra.Command, _ []string) error {
	out, _ := cmd.Flags().GetString("out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", out, err)
	}
	// DisableAutoGenTag убирает строку с датой генерации — даёт стабильный diff.
	rootCmd.DisableAutoGenTag = true
	if err := doc.GenMarkdownTree(rootCmd, out); err != nil {
		return fmt.Errorf("gen markdown: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✓ docs generated in %s\n", out)
	return nil
}
