/*
TODO: https://github.com/redis/rueidis 가 더 나을지
*/
package main

import (
	"fmt"
	"os"

	"redisctl/cmd"
	"redisctl/internal/config"
	"redisctl/internal/styles"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"
	commit  = "Seokwon Choi"
	date    = "unknown"
)

func main() {
	// Initialize logger with beautiful styling
	log.SetDefault(log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    false,
		ReportTimestamp: false,
		Prefix:          "redisctl",
	}))

	// Initialize global config
	config.Init()

	rootCmd := &cobra.Command{
		Use:   "redisctl",
		Short: " Redis Cluster Management CLI Tool",
		Long: styles.TitleStyle.Render("Redis Cluster Management CLI Tool") + "\n\n" +
			styles.DescStyle.Render("Redis 클러스터를 관리하기 위한 도구입니다.") + "\n" +
			styles.DescStyle.Render("클러스터 생성, 노드 관리, 리샤딩, 상태 확인 등을 지원합니다."),
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Global flags setup
			user, _ := cmd.Flags().GetString("user")
			password, _ := cmd.Flags().GetString("password")

			config.SetAuth(user, password)
		},
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true, // hides default "completion" command
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringP("user", "u", "", "Redis 사용자명 (기본 인증 사용시 생략 가능)")
	rootCmd.PersistentFlags().StringP("password", "p", "", "Redis 비밀번호 (필수)")

	// subcommands
	rootCmd.AddCommand(
		cmd.NewCreateCommand(),
		cmd.NewAddNodeCommand(),
		cmd.NewReshardCommand(),
		cmd.NewDelNodeCommand(),
		cmd.NewCheckCommand(),
		cmd.NewPopulateCommand(),
		cmd.NewRebalanceCommand(),
		cmd.NewConfigCommand(),
		cmd.NewVersionCommand(version, commit, date),
	)

	// help
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "help [command]",
		Short:  "h 명령어에 대한 도움말을 표시합니다",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				rootCmd.Help()
			} else {
				if helpCmd, _, err := rootCmd.Find(args); err == nil {
					helpCmd.Help()
				} else {
					fmt.Printf("명령어를 찾을 수 없습니다: %s\n", args[0])
				}
			}
		},
	})

	if err := rootCmd.Execute(); err != nil {
		log.Error("명령 실행 실패", "error", err)
		os.Exit(1)
	}
}
