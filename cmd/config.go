/*
TODO: config를 좀 더 쓸모있게?
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"redisctl/internal/config"
	"redisctl/internal/styles"
)

// NewConfigCommand config 명령어
func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "c 설정 정보를 표시합니다",
		Long: styles.TitleStyle.Render("[C] 설정 정보 표시") + "\n\n" +
			styles.DescStyle.Render("현재 설정된 redisctl 구성 정보를 표시합니다.") + "\n" +
			styles.DescStyle.Render("환경 변수 및 기본값을 포함한 모든 설정을 확인할 수 있습니다.") + "\n\n" +
			styles.DescStyle.Render("지원하는 환경 변수:") + "\n" +
			styles.DescStyle.Render("• REDIS_USER - Redis 사용자명") + "\n" +
			styles.DescStyle.Render("• REDIS_PASSWORD - Redis 비밀번호") + "\n" +
			styles.DescStyle.Render("• REDIS_CONNECT_TIMEOUT - 연결 타임아웃 (예: 10s)") + "\n" +
			styles.DescStyle.Render("• REDIS_COMMAND_TIMEOUT - 명령 타임아웃 (예: 60s)") + "\n" +
			styles.DescStyle.Render("• REDIS_MAX_RETRIES - 최대 재시도 횟수") + "\n" +
			styles.DescStyle.Render("• REDIS_POOL_SIZE - 연결 풀 크기") + "\n" +
			styles.DescStyle.Render("• REDIS_DEBUG - 디버그 모드 (true/1)"),
		Example: `  # 현재 설정 표시
  redisctl config

  # 환경 변수 설정 후 확인
  set REDIS_USER=admin
  set REDIS_PASSWORD=mypassword
  set REDIS_DEBUG=true
  redisctl config`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShowConfig()
		},
	}

	return cmd
}

func runShowConfig() error {
	fmt.Println(styles.InfoStyle.Render("redisctl 설정 정보"))
	fmt.Println()

	// 현재 auth 설정
	user, password := config.GetAuth()

	fmt.Println(styles.TitleStyle.Render("인증 정보"))
	fmt.Println(styles.DescStyle.Render("(CLI 플래그가 환경 변수보다 우선순위가 높습니다)"))

	if user != "" {
		fmt.Printf("사용자명: %s\n", styles.HighlightStyle.Render(user))
	} else {
		fmt.Printf("사용자명: %s\n", styles.DescStyle.Render("<설정되지 않음>"))
	}

	if password != "" {
		fmt.Printf("비밀번호: %s\n", styles.SuccessStyle.Render("***설정됨***"))
	} else {
		fmt.Printf("비밀번호: %s\n", styles.ErrorStyle.Render("<설정되지 않음>"))
	}

	fmt.Println()
	fmt.Println(styles.TitleStyle.Render("환경 변수 상태"))
	envUser := os.Getenv("REDIS_USER")
	envPassword := os.Getenv("REDIS_PASSWORD")

	if envUser != "" {
		fmt.Printf("REDIS_USER: %s\n", styles.HighlightStyle.Render(envUser))
	} else {
		fmt.Printf("REDIS_USER: %s\n", styles.DescStyle.Render("<설정되지 않음>"))
	}

	if envPassword != "" {
		fmt.Printf("REDIS_PASSWORD: %s\n", styles.SuccessStyle.Render("***설정됨***"))
	} else {
		fmt.Printf("REDIS_PASSWORD: %s\n", styles.DescStyle.Render("<설정되지 않음>"))
	}

	fmt.Println()
	fmt.Println(styles.TitleStyle.Render("연결 설정"))
	fmt.Printf("연결 타임아웃: %s\n", styles.HighlightStyle.Render(config.GetConnectTimeout().String()))
	fmt.Printf("명령 타임아웃: %s\n", styles.HighlightStyle.Render(config.GetCommandTimeout().String()))
	fmt.Printf("최대 재시도: %s\n", styles.HighlightStyle.Render(fmt.Sprintf("%d", config.GetMaxRetries())))
	fmt.Printf("연결 풀 크기: %s\n", styles.HighlightStyle.Render(fmt.Sprintf("%d", config.GetPoolSize())))

	fmt.Println()
	fmt.Println(styles.TitleStyle.Render("! 디버그 설정"))
	if config.IsDebugEnabled() {
		fmt.Printf("디버그 모드: %s\n", styles.SuccessStyle.Render("활성화"))
	} else {
		fmt.Printf("디버그 모드: %s\n", styles.DescStyle.Render("비활성화"))
	}

	fmt.Println()
	fmt.Println(styles.TitleStyle.Render("환경 변수 도움말"))
	fmt.Println(styles.DescStyle.Render("다음 환경 변수로 설정을 변경할 수 있습니다:"))
	fmt.Println()
	fmt.Printf("  %s - Redis 사용자명\n", styles.HighlightStyle.Render("REDIS_USER"))
	fmt.Printf("  %s - Redis 비밀번호\n", styles.HighlightStyle.Render("REDIS_PASSWORD"))
	fmt.Printf("  %s - 연결 타임아웃 (예: 10s, 30s)\n", styles.HighlightStyle.Render("REDIS_CONNECT_TIMEOUT"))
	fmt.Printf("  %s - 명령 타임아웃 (예: 60s, 120s)\n", styles.HighlightStyle.Render("REDIS_COMMAND_TIMEOUT"))
	fmt.Printf("  %s - 최대 재시도 횟수 (예: 3, 5)\n", styles.HighlightStyle.Render("REDIS_MAX_RETRIES"))
	fmt.Printf("  %s - 연결 풀 크기 (예: 10, 20)\n", styles.HighlightStyle.Render("REDIS_POOL_SIZE"))
	fmt.Printf("  %s - 디버그 모드 (true/1)\n", styles.HighlightStyle.Render("REDIS_DEBUG"))

	return nil
}
