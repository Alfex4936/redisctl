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

	// 기존 노드에 연결 시도
	fmt.Println(styles.InfoStyle.Render("1단계: 기존 클러스터 연결 확인 중..."))
	fmt.Printf("  %s 연결 중...", existingNode)

	existingClient, err := cm.Connect(existingNode)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("연결 실패"))
		return fmt.Errorf("기존 노드 %s 연결 실패: %w", existingNode, err)
	}

	// 클러스터 일부인지 체크
	clusterInfo, err := cm.GetClusterInfo(existingNode)
	if err != nil || clusterInfo["cluster_state"] == "fail" {
		fmt.Printf(" %s\n", styles.RenderError("클러스터 상태 비정상"))
		return fmt.Errorf("기존 노드가 정상적인 클러스터에 속하지 않습니다")
	}

	fmt.Printf(" %s\n", styles.RenderSuccess("연결 성공"))

	fmt.Println(styles.InfoStyle.Render("2단계: 새 노드 연결 확인 중..."))
	fmt.Printf("  %s 연결 중...", newNode)

	newClient, err := cm.Connect(newNode)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("연결 실패"))
		return fmt.Errorf("새 노드 %s 연결 실패: %w", newNode, err)
	}

	// 이미 다른 클러스터에 있나? (에러 메시지 간소화)
	newClusterInfo, err := cm.GetClusterInfo(newNode)
	if err == nil && newClusterInfo["cluster_state"] != "fail" {
		fmt.Printf(" %s\n", styles.RenderError("이미 클러스터에 참여 중"))

		host, port, parseErr := parseNodeAddress(newNode)
		if parseErr != nil {
			return fmt.Errorf("노드 주소 파싱 실패: %w", parseErr)
		}

		return fmt.Errorf(`노드 %s는 이미 다른 클러스터에 참여 중입니다

 해결 방법:
   redis-cli -h %s -p %s cluster reset hard

 주의: 이 명령은 노드의 모든 클러스터 데이터를 삭제합니다`,
			newNode, host, port)
	}

	fmt.Printf(" %s\n", styles.RenderSuccess("연결 성공"))

	// master-id가 지정된 경우 마스터 노드 검증
	if masterID != "" {
		fmt.Println(styles.InfoStyle.Render("3단계: 마스터 노드 검증 중..."))
		fmt.Printf("  마스터 ID %s 확인 중...", masterID)

		clusterNodes, err := cm.GetClusterNodes(existingNode)
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderError("클러스터 노드 조회 실패"))
			return fmt.Errorf("클러스터 노드 정보 조회 실패: %w", err)
		}

		masterFound := false
		var masterNode redis.ClusterNode
		for _, node := range clusterNodes {
			if node.ID == masterID {
				masterFound = true
				masterNode = node
				break
			}
		}

		if !masterFound {
			fmt.Printf(" %s\n", styles.RenderError("마스터 ID를 찾을 수 없음"))
			return fmt.Errorf("지정된 마스터 ID %s를 클러스터에서 찾을 수 없습니다", masterID)
		}

		// 진짜 마스터인지 확인
		isMaster := false
		for _, flag := range masterNode.Flags {
			if flag == "master" {
				isMaster = true
				break
			}
		}

		if !isMaster {
			fmt.Printf(" %s\n", styles.RenderError("마스터 노드가 아님"))
			return fmt.Errorf("지정된 노드 ID %s는 마스터 노드가 아닙니다", masterID)
		}

		fmt.Printf(" %s\n", styles.RenderSuccess("마스터 노드 확인됨"))
	}

	fmt.Println(styles.InfoStyle.Render("4단계: 클러스터에 노드 추가 중..."))
	fmt.Printf("  %s를 클러스터에 추가 중...", newNode)

	host, port, err := parseNodeAddress(newNode)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("주소 파싱 실패"))
		return fmt.Errorf("새 노드 주소 파싱 실패: %w", err)
	}

	// CLUSTER MEETㅇ로 노드 추가
	err = existingClient.ClusterMeet(ctx, host, port).Err()
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("CLUSTER MEET 실패"))
		return fmt.Errorf("CLUSTER MEET 명령 실패: %w", err)
	}

	fmt.Printf(" %s\n", styles.RenderSuccess("클러스터 참여 완료"))

	// 마스터 아디가 지정된 경우 복제본 설정
	if masterID != "" {
		fmt.Println(styles.InfoStyle.Render("5단계: 복제본 설정 중..."))

		// 잠깐 기다려야지... 클러스터 상태 안정화 (동적 대기로 개선)
		fmt.Print("  클러스터 상태 안정화 대기 중...")
		err := waitForClusterStable(cm, existingNode, 10*time.Second)
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderWarning("타임아웃 - 하드코딩 대기로 fallback"))
			time.Sleep(3 * time.Second)
		} else {
			fmt.Printf(" %s\n", styles.RenderSuccess("완료"))
		}

		fmt.Printf("  %s를 %s의 복제본으로 설정 중...", newNode, masterID)

		// 복제본 설정 재시도 로직 (마스터 노드 정보 동기화 대기)
		maxRetries := 5
		var replicateErr error

		for retry := 0; retry < maxRetries; retry++ {
			if retry > 0 {
				// 재시도 전 추가 대기 (클러스터 동기화를 위해)
				time.Sleep(time.Second * 3)
				fmt.Printf("\n    재시도 %d/%d...", retry+1, maxRetries)
			}

			replicateErr = newClient.ClusterReplicate(ctx, masterID).Err()
			if replicateErr == nil {
				break // 성공
			}

			// "Unknown node" 오류인 경우 추가 대기 후 재시도
			if strings.Contains(replicateErr.Error(), "Unknown node") && retry < maxRetries-1 {
				fmt.Printf(" (마스터 노드 동기화 대기 중)")
				continue
			}
		}

		if replicateErr != nil {
			fmt.Printf(" %s\n", styles.RenderError("복제 설정 실패"))

			// 디버그 정보 제공
			fmt.Printf("\n디버그 정보:\n")
			fmt.Printf("  마스터 ID: %s\n", masterID)
			fmt.Printf("  오류: %v\n", replicateErr)

			// 새 노드에서 클러스터 노드 목록 확인 시도
			if clusterNodes, err := newClient.ClusterNodes(ctx).Result(); err == nil {
				nodeCount := len(strings.Split(clusterNodes, "\n")) - 1 // 빈 줄 제외
				fmt.Printf("  새 노드가 인식한 클러스터 노드 수: %d\n", nodeCount)
			}

			// 복제 설정 실패 시 롤백
			rollbackAddNode(cm, newNode, existingNode)
			return fmt.Errorf("CLUSTER REPLICATE 명령 실패: %w", replicateErr)
		}

		fmt.Printf(" %s\n", styles.RenderSuccess("복제본 설정 완료"))
	}

	fmt.Println(styles.InfoStyle.Render("6단계: 노드 추가 확인 중..."))

	// 동적 대기로 개선 (성능 최적화)
	fmt.Print("  클러스터 상태 재확인 중...")
	err = waitForClusterStable(cm, existingNode, 8*time.Second)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderWarning("타임아웃 - 하드코딩 대기로 fallback"))
		time.Sleep(2 * time.Second)
	} else {
		fmt.Printf(" %s\n", styles.RenderSuccess("완료"))
	}

	// 새로 추가된 노드에서 직접 노드 ID 조회 (더 안정적)
	fmt.Print("  새 노드 정보 조회 중...")

	var addedNode *redis.ClusterNode

	// 방법 1: 새 노드에서 직접 ID 조회
	newNodeID, err := newClient.ClusterMyID(ctx).Result()
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderWarning("직접 조회 실패"))

		// 방법 2: 클러스터 노드 목록에서 검색
		clusterNodes, err := cm.GetClusterNodes(existingNode)
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderError("실패"))
			return fmt.Errorf("클러스터 상태 확인 실패: %w", err)
		}

		// 새 노드 찾기
		newHost, newPort, _ := parseNodeAddress(newNode)
		expectedAddress := fmt.Sprintf("%s:%s", newHost, newPort)

		for _, node := range clusterNodes {
			// 클러스터 노드 주소 정규화 (busport 제거)
			nodeAddress := normalizeClusterAddress(node.Address)

			if nodeAddress == expectedAddress {
				addedNode = &node
				break
			}
		}

		if addedNode == nil {
			fmt.Printf(" %s\n", styles.RenderError("실패"))
			return fmt.Errorf("추가된 노드를 클러스터에서 찾을 수 없습니다")
		}
	} else {
		// 직접 조회 성공 - 클러스터 노드 목록에서 해당 ID로 검색
		// 복제본 설정이 있었다면 최신 정보를 위해 잠시 대기
		if masterID != "" {
			time.Sleep(time.Second * 2)
		}

		// 클러스터 동기화를 위해 재시도 로직 추가
		maxRetries := 5
		for retry := 0; retry < maxRetries; retry++ {
			clusterNodes, err := cm.GetClusterNodes(existingNode)
			if err != nil {
				fmt.Printf(" %s\n", styles.RenderError("실패"))
				return fmt.Errorf("클러스터 상태 확인 실패: %w", err)
			}

			for _, node := range clusterNodes {
				if node.ID == newNodeID {
					addedNode = &node
					break
				}
			}

			if addedNode != nil {
				break // 노드를 찾았으면 반복 중단
			}

			// 노드를 못 찾았으면 잠시 대기 후 재시도
			if retry < maxRetries-1 {
				time.Sleep(time.Second * 2)
			}
		}

		if addedNode == nil {
			// 최종적으로 못 찾으면 주소 기반 검색으로 fallback
			fmt.Printf(" %s\n", styles.RenderWarning("ID 검색 실패 - 주소 기반 검색으로 fallback"))

			clusterNodes, err := cm.GetClusterNodes(existingNode)
			if err != nil {
				return fmt.Errorf("클러스터 상태 확인 실패: %w", err)
			}

			newHost, newPort, _ := parseNodeAddress(newNode)
			expectedAddress := fmt.Sprintf("%s:%s", newHost, newPort)

			for _, node := range clusterNodes {
				nodeAddress := normalizeClusterAddress(node.Address)
				if nodeAddress == expectedAddress {
					addedNode = &node
					break
				}
			}

			if addedNode == nil {
				fmt.Printf(" %s\n", styles.RenderError("실패"))

				// 디버그 정보 제공
				fmt.Printf("\n디버그 정보:\n")
				fmt.Printf("  새 노드 ID: %s\n", newNodeID)
				fmt.Printf("  예상 주소: %s\n", expectedAddress)
				fmt.Printf("  클러스터 내 노드 수: %d\n", len(clusterNodes))

				// 최소한의 노드 정보라도 표시하기 위해 직접 구성
				fmt.Printf("\n%s\n", styles.RenderWarning("클러스터 뷰에서 노드를 찾을 수 없지만 추가는 완료되었습니다."))

				// 직접 노드 정보 구성
				directNodeInfo := styles.SubtitleStyle.Render("추가된 노드 정보 (직접 조회)") + "\n" +
					fmt.Sprintf("• 노드 ID: %s\n", newNodeID) +
					fmt.Sprintf("• 주소: %s\n", newNode) +
					fmt.Sprintf("• 역할: %s\n", func() string {
						if masterID != "" {
							return "복제본"
						}
						return "마스터"
					}()) +
					fmt.Sprintf("• 슬롯: %s\n", func() string {
						if masterID != "" {
							return "해당 없음 (복제본)"
						}
						return "0개 (새 마스터는 슬롯이 없습니다)"
					}())

				fmt.Println()
				fmt.Println(styles.RenderSuccess("노드가 성공적으로 클러스터에 추가되었습니다!"))
				fmt.Println()
				fmt.Println(styles.BoxStyle.Render(directNodeInfo))

				if masterID == "" {
					fmt.Println()
					fmt.Println(styles.InfoStyle.Render("참고: 새 마스터 노드에는 슬롯이 할당되지 않았습니다."))
					fmt.Println(styles.DescStyle.Render("   슬롯을 할당하려면 'reshard' 명령을 사용하세요."))
				}

				return nil // 성공으로 처리
			}
		}
	}

	fmt.Printf(" %s\n", styles.RenderSuccess("완료"))

	// Success message
	fmt.Println()
	fmt.Println(styles.RenderSuccess("노드가 성공적으로 클러스터에 추가되었습니다!"))
	fmt.Println()

	// 노드 정보
	nodeInfo := styles.SubtitleStyle.Render("추가된 노드 정보") + "\n" +
		fmt.Sprintf("• 노드 ID: %s\n", addedNode.ID) +
		fmt.Sprintf("• 주소: %s\n", addedNode.Address) +
		fmt.Sprintf("• 역할: %s\n", getNodeRole(addedNode.Flags))

	if masterID != "" {
		nodeInfo += fmt.Sprintf("• 마스터: %s\n", addedNode.Master)
	} else {
		nodeInfo += fmt.Sprintf("• 슬롯: %d개 (새 마스터는 슬롯이 없습니다)\n", len(addedNode.Slots))
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
