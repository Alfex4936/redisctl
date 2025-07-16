package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// TODO: 유용한 명령어들?

// NewVersionCommand version 명령어
func NewVersionCommand(version, commit, date string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "v 버전 정보를 표시합니다",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("redisctl %s (commit: %s, built: %s)\n", version, commit, date)
		},
	}
}
