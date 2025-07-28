package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"redisctl/internal/config"
	"redisctl/internal/redis"
	"redisctl/internal/styles"

	"github.com/spf13/cobra"
)

// NewAddNodeCommand add-node 명령어
func NewAddNodeCommand() *cobra.Command {
	var masterID string

	cmd := &cobra.Command{
		Use:   "add-node [--master-id <str>] new_ip:new_port existing_ip:existing_port",
		Short: "+ 클러스터에 새 노드를 추가합니다",
		Long: styles.TitleStyle.Render("[+] 클러스터 노드 추가") + "\n\n" +
			styles.DescStyle.Render("기존 Redis 클러스터에 새로운 노드를 추가합니다.") + "\n" +
			styles.DescStyle.Render("--master-id가 지정되면 해당 마스터의 복제본으로, 생략되면 마스터로 추가됩니다."),
		Example: `  # 새 마스터 노드 추가 (슬롯 없음)
  redisctl add-node localhost:7007 localhost:7001

  # 특정 마스터의 복제본으로 노드 추가
  redisctl add-node --master-id <master-node-id> localhost:7008 localhost:7001`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.ValidateAuth(); err != nil {
				return err
			}

			return runAddNode(args[0], args[1], masterID)
		},
	}

	cmd.Flags().StringVar(&masterID, "master-id", "", "새 노드를 이 마스터의 복제본으로 만듭니다")
	return cmd
}

func runAddNode(newNode, existingNode, masterID string) error {
	fmt.Println(styles.InfoStyle.Render("노드 추가 시작..."))
	fmt.Printf("새 노드: %s\n", newNode)
	fmt.Printf("기존 노드: %s\n", existingNode)

	if masterID != "" {
		fmt.Printf("복제본으로 추가 (마스터 ID: %s)\n", masterID)
	} else {
		fmt.Println("마스터로 추가 (슬롯 없음)")
	}

	user, password := config.GetAuth()
	cm := redis.NewClusterManager(user, password)
	defer cm.Close()

	ctx := context.Background()

	// 1단계: 기존 클러스터 검증
	fmt.Println(styles.InfoStyle.Render("1단계: 기존 클러스터 검증 중..."))
	fmt.Printf("  %s 연결 및 클러스터 상태 확인...", existingNode)

	_, err := cm.Connect(existingNode)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("연결 실패"))
		return fmt.Errorf("기존 노드 %s 연결 실패: %w", existingNode, err)
	}

	clusterInfo, err := cm.GetClusterInfo(existingNode)
	if err != nil || clusterInfo["cluster_state"] == "fail" {
		fmt.Printf(" %s\n", styles.RenderError("클러스터 상태 비정상"))
		return fmt.Errorf("기존 노드가 정상적인 클러스터에 속하지 않습니다")
	}

	fmt.Printf(" %s\n", styles.RenderSuccess("검증 완료"))

	// 2단계: 새 노드 검증
	fmt.Println(styles.InfoStyle.Render("2단계: 새 노드 검증 중..."))
	fmt.Printf("  %s 연결 및 빈 노드 확인...", newNode)

	newClient, err := cm.Connect(newNode)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("연결 실패"))
		return fmt.Errorf("새 노드 %s 연결 실패: %w", newNode, err)
	}

	// 새 노드가 이미 클러스터에 속해있는지 확인
	newClusterInfo, err := cm.GetClusterInfo(newNode)
	if err == nil && newClusterInfo["cluster_state"] != "fail" {
		fmt.Printf(" %s\n", styles.RenderError("이미 클러스터에 참여 중"))
		host, port, _ := parseNodeAddress(newNode)
		return fmt.Errorf(`노드 %s는 이미 다른 클러스터에 참여 중입니다

 해결 방법:
   redis-cli -h %s -p %s cluster reset hard

 주의: 이 명령은 노드의 모든 클러스터 데이터를 삭제합니다`,
			newNode, host, port)
	}

	fmt.Printf(" %s\n", styles.RenderSuccess("검증 완료"))

	// 3단계: 마스터 노드 검증 (복제본인 경우만)
	if masterID != "" {
		fmt.Println(styles.InfoStyle.Render("3단계: 마스터 노드 검증 중..."))
		fmt.Printf("  마스터 ID %s 확인...", masterID)

		clusterNodes, err := cm.GetClusterNodes(existingNode)
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderError("클러스터 노드 조회 실패"))
			return fmt.Errorf("클러스터 노드 정보 조회 실패: %w", err)
		}

		masterFound := false
		for _, node := range clusterNodes {
			if node.ID == masterID {
				// 마스터 노드인지 확인
				for _, flag := range node.Flags {
					if flag == "master" {
						masterFound = true
						break
					}
				}
				break
			}
		}

		if !masterFound {
			fmt.Printf(" %s\n", styles.RenderError("마스터 노드를 찾을 수 없음"))
			return fmt.Errorf("지정된 마스터 ID %s는 존재하지 않거나 마스터 노드가 아닙니다", masterID)
		}

		fmt.Printf(" %s\n", styles.RenderSuccess("마스터 노드 확인됨"))
	}

	// 4단계: CLUSTER MEET으로 노드 추가 (Redis 네이티브 방식)
	fmt.Println(styles.InfoStyle.Render("4단계: 클러스터에 노드 추가 중..."))
	fmt.Printf("  CLUSTER MEET 실행 중...")

	host, port, err := parseNodeAddress(newNode)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("주소 파싱 실패"))
		return fmt.Errorf("새 노드 주소 파싱 실패: %w", err)
	}

	err = newClient.ClusterMeet(ctx, host, port).Err()
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("CLUSTER MEET 실패"))
		return fmt.Errorf("CLUSTER MEET 명령 실패: %w", err)
	}

	fmt.Printf(" %s\n", styles.RenderSuccess("노드 추가 완료"))

	// 5단계: 복제본 설정 (필요한 경우만)
	if masterID != "" {
		fmt.Println(styles.InfoStyle.Render("5단계: 복제본 설정 중..."))

		// Redis 네이티브 방식: 1초 대기 후 클러스터 수렴 대기
		fmt.Print("  클러스터 수렴 대기 중...")
		time.Sleep(1 * time.Second)

		err := waitForClusterJoin(cm, existingNode, 30*time.Second)
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderWarning("타임아웃"))
			fmt.Println(styles.WarningStyle.Render("  경고: 클러스터 수렴을 기다리다 타임아웃되었습니다. 복제본 설정을 시도합니다."))
		} else {
			fmt.Printf(" %s\n", styles.RenderSuccess("수렴 완료"))
		}

		fmt.Printf("  복제본 설정 중...")
		err = newClient.ClusterReplicate(ctx, masterID).Err()
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderError("복제 설정 실패"))
			return fmt.Errorf("CLUSTER REPLICATE 명령 실패: %w", err)
		}

		fmt.Printf(" %s\n", styles.RenderSuccess("복제본 설정 완료"))
	}

	// 6단계: 최종 확인
	fmt.Println(styles.InfoStyle.Render("6단계: 추가 완료 확인 중..."))
	fmt.Print("  새 노드 정보 조회...")

	newNodeID, err := newClient.ClusterMyID(ctx).Result()
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("실패"))
		return fmt.Errorf("새 노드 ID 조회 실패: %w", err)
	}

	fmt.Printf(" %s\n", styles.RenderSuccess("완료"))

	// 성공 메시지
	fmt.Println()
	fmt.Println(styles.RenderSuccess("노드가 성공적으로 클러스터에 추가되었습니다!"))
	fmt.Println()

	// 추가된 노드 정보 표시
	nodeInfo := styles.SubtitleStyle.Render("추가된 노드 정보") + "\n" +
		fmt.Sprintf("• 노드 ID: %s\n", newNodeID) +
		fmt.Sprintf("• 주소: %s\n", newNode) +
		fmt.Sprintf("• 역할: %s\n", func() string {
			if masterID != "" {
				return "복제본"
			}
			return "마스터"
		}())

	if masterID != "" {
		nodeInfo += fmt.Sprintf("• 마스터: %s\n", masterID)
	} else {
		nodeInfo += "• 슬롯: 0개 (새 마스터는 슬롯이 없습니다)\n"
	}

	fmt.Println(styles.BoxStyle.Render(nodeInfo))

	if masterID == "" {
		fmt.Println()
		fmt.Println(styles.InfoStyle.Render("참고: 새 마스터 노드에는 슬롯이 할당되지 않았습니다."))
		fmt.Println(styles.DescStyle.Render("   슬롯을 할당하려면 'reshard' 명령을 사용하세요."))
	}

	return nil
}

// add-node 롤백 함수 (CLUSTER MEET 성공 후 CLUSTER REPLICATE 실패 시)
func rollbackAddNode(cm *redis.ClusterManager, newNode, existingNode string) {
	fmt.Println(styles.WarningStyle.Render("노드 추가 실패 - 롤백 중..."))

	// 새 노드를 클러스터에서 제거하기 위해 CLUSTER RESET 실행
	fmt.Printf("  %s 초기화 중...", newNode)

	newClient, err := cm.Connect(newNode)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderWarning("연결 실패"))
		return
	}

	// 새 노드 리셋
	ctx := context.Background()
	err = newClient.Do(ctx, "CLUSTER", "RESET", "HARD").Err()
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderWarning("실패"))
	} else {
		fmt.Printf(" %s\n", styles.RenderSuccess("완료"))
	}

	// 기존 클러스터에서 새 노드를 잊게 하기 (best effort)
	existingClient, err := cm.Connect(existingNode)
	if err == nil {
		// 새 노드의 ID를 얻기 위해 클러스터 노드 조회 시도
		clusterNodes, err := cm.GetClusterNodes(existingNode)
		if err == nil {
			newHost, newPort, _ := parseNodeAddress(newNode)
			expectedAddress := fmt.Sprintf("%s:%s", newHost, newPort)

			for _, node := range clusterNodes {
				nodeAddress := normalizeClusterAddress(node.Address)
				if nodeAddress == expectedAddress {
					// CLUSTER FORGET으로 새 노드 제거
					existingClient.Do(ctx, "CLUSTER", "FORGET", node.ID).Err()
					break
				}
			}
		}
	}
}

// waitForClusterJoin waits for all nodes in the cluster to have consistent configuration
// This is Redis's native approach: wait for global cluster consistency
func waitForClusterJoin(cm *redis.ClusterManager, existingNode string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Get all nodes from the existing cluster node perspective
		clusterNodes, err := cm.GetClusterNodes(existingNode)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// Check if all nodes have consistent view of the cluster
		consistent := true
		var firstSignature string

		for i, node := range clusterNodes {
			// Skip nodes that are unreachable or have connection issues
			if stringSliceContains(node.Flags, "fail") || stringSliceContains(node.Flags, "disconnected") {
				continue
			}

			nodeAddr := node.Address
			if nodeAddr == "" {
				continue
			}

			nodeClusterNodes, err := cm.GetClusterNodes(nodeAddr)
			if err != nil {
				// If we can't reach this node, wait and try again
				consistent = false
				break
			}

			// Create a simple signature from the cluster nodes info
			signature := createClusterSignature(nodeClusterNodes)
			if i == 0 {
				firstSignature = signature
			} else if firstSignature != signature {
				consistent = false
				break
			}
		}

		if consistent {
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for cluster to join")
}

// createClusterSignature creates a simple signature from cluster nodes info
// This mimics Redis's config signature approach
func createClusterSignature(clusterNodes []redis.ClusterNode) string {
	var parts []string
	for _, node := range clusterNodes {
		// Include node ID and flags for signature
		flagStr := strings.Join(node.Flags, ",")
		parts = append(parts, fmt.Sprintf("%s:%s", node.ID, flagStr))
	}
	// Sort to ensure consistent ordering
	return strings.Join(parts, "|")
}

// Helper function to check if a slice contains a string
func stringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
