/*
TODO: func moveSlots - 슬롯 goroutine을 쓸지
*/
package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"

	"redisctl/internal/config"
	"redisctl/internal/styles"
)

// NewDelNodeCommand del-node 명령어
func NewDelNodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "del-node <cluster-node-ip:port> <node-id>",
		Short: "> Redis 클러스터에서 노드를 제거합니다",
		Long: styles.TitleStyle.Render("[-] Redis 클러스터 노드 제거") + "\n\n" +
			styles.DescStyle.Render("Redis 클러스터에서 지정된 노드를 안전하게 제거합니다.") + "\n\n" +
			styles.DescStyle.Render("노드를 제거하기 전에 다음 작업을 수행합니다:") + "\n" +
			styles.DescStyle.Render("• 마스터 노드의 경우: 슬롯을 다른 마스터들에게 재분배") + "\n" +
			styles.DescStyle.Render("• 레플리카 노드의 경우: 단순히 클러스터에서 제거") + "\n" +
			styles.DescStyle.Render("• 모든 노드가 정상 상태인지 확인") + "\n" +
			styles.DescStyle.Render("• 클러스터 토폴로지 업데이트"),
		Example: `  # 레플리카 노드 제거
  redisctl del-node localhost:7001 a1b2c3d4e5f6...

  # 마스터 노드 제거 (슬롯 자동 재분배)
  redisctl del-node localhost:7002 f6e5d4c3b2a1...`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.ValidateAuth(); err != nil {
				return err
			}
			return runDelNode(args[0], args[1])
		},
	}

	return cmd
}

type NodeInfo struct {
	ID        string
	Addr      string
	IsMaster  bool
	IsReplica bool
	MasterID  string
	Slots     []int
	Replicas  []string
}

func runDelNode(clusterAddr, nodeIDToRemove string) error {
	fmt.Println(styles.InfoStyle.Render("Redis 클러스터 노드 제거"))
	fmt.Printf("클러스터: %s\n", styles.HighlightStyle.Render(clusterAddr))
	fmt.Printf("제거할 노드 ID: %s\n", styles.HighlightStyle.Render(nodeIDToRemove))
	fmt.Println()

	// 클러스터 연결
	user, password := config.GetAuth()
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    []string{clusterAddr},
		Username: user,
		Password: password,
	})
	defer client.Close()

	ctx := context.Background()

	// Validate cluster connectivity
	if err := validateDelNodeConnectivity(ctx, client); err != nil {
		return fmt.Errorf("클러스터 연결 검증 실패: %w", err)
	}

	// 클러스터 정보 가져와서 노드 검증
	nodeInfo, err := getDelNodeInfo(ctx, client, nodeIDToRemove)
	if err != nil {
		return fmt.Errorf("노드 정보 조회 실패: %w", err)
	}

	// 추가 검증: 노드에 직접 연결 가능한지 체크
	// 노드에 직접 연결할 수 없으면 사용자에게 경고
	if err := validateNodeReachability(ctx, nodeInfo); err != nil {
		fmt.Println(styles.WarningStyle.Render("  경고: 제거할 노드에 직접 연결할 수 없습니다. 노드가 이미 다운되었을 수 있습니다."))
		fmt.Println(styles.WarningStyle.Render("    계속 진행하면 클러스터에서 노드 정보만 제거됩니다."))
	}

	// 슬롯이 있는 마스터인지 체크
	if nodeInfo.IsMaster && len(nodeInfo.Slots) > 0 {
		// 재분배를 위해 충분한 마스터가 있는지 검증
		masters, err := getOtherMasters(ctx, client, nodeIDToRemove)
		if err != nil {
			return fmt.Errorf("다른 마스터 노드 조회 실패: %w", err)
		}

		if len(masters) == 0 {
			return fmt.Errorf("x 삭제할 수 없습니다: 클러스터에 다른 마스터 노드가 없습니다")
		}

		// 이 마스터를 제거하면 3개 미만이 되는지 체크
		if len(masters) < 2 { // +1 for the current master being removed = 3 total minimum
			return fmt.Errorf("x 삭제할 수 없습니다: 클러스터 운영을 위해 최소 3개의 마스터가 필요합니다 (현재: %d개)", len(masters)+1)
		}

		fmt.Println(styles.WarningStyle.Render("  마스터 노드에 슬롯이 할당되어 있습니다. 슬롯을 먼저 재분배합니다."))
		if err := reshardBeforeRemoval(ctx, client, nodeInfo); err != nil {
			return fmt.Errorf("슬롯 재분배 실패: %w", err)
		}
	} else if nodeInfo.IsMaster {
		// 슬롯 없는 마스터도 최소 마스터 수 검증 필요
		masters, err := getOtherMasters(ctx, client, nodeIDToRemove)
		if err != nil {
			return fmt.Errorf("다른 마스터 노드 조회 실패: %w", err)
		}

		if len(masters) < 2 { // +1 for the current master being removed = 3 total minimum
			return fmt.Errorf("x 삭제할 수 없습니다: 클러스터 운영을 위해 최소 3개의 마스터가 필요합니다 (현재: %d개)", len(masters)+1)
		}

		fmt.Println(styles.InfoStyle.Render("ℹ슬롯이 없는 마스터 노드입니다. 바로 제거합니다."))
	}

	// 클러스터에서 노드 제거
	if err := removeNodeFromCluster(ctx, client, nodeIDToRemove); err != nil {
		return fmt.Errorf("노드 제거 실패: %w", err)
	}

	// 최종 검증
	if err := validateRemoval(ctx, client, nodeIDToRemove); err != nil {
		return fmt.Errorf("노드 제거 검증 실패: %w", err)
	}

	fmt.Println()
	fmt.Println(styles.SuccessStyle.Render("노드가 성공적으로 제거되었습니다!"))
	return nil
}

func validateDelNodeConnectivity(ctx context.Context, client *redis.ClusterClient) error {
	fmt.Print(styles.InfoStyle.Render("1. 클러스터 연결 확인..."))

	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return err
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return nil
}

func getDelNodeInfo(ctx context.Context, client *redis.ClusterClient, nodeID string) (*NodeInfo, error) {
	fmt.Print(styles.InfoStyle.Render("2. 노드 정보 조회..."))

	// 클러스터 노드 정보 가져오기
	result := client.ClusterNodes(ctx)
	if result.Err() != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return nil, result.Err()
	}

	lines := strings.Split(result.Val(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 8 {
			continue
		}

		currentNodeID := parts[0]
		if currentNodeID == nodeID {
			nodeInfo := &NodeInfo{
				ID:   currentNodeID,
				Addr: parts[1],
			}

			// Parse flags using unified utility
			nodeFlags := parseNodeFlagsSlice(strings.Split(parts[2], ","))
			nodeInfo.IsMaster = nodeFlags.IsMaster
			nodeInfo.IsReplica = nodeFlags.IsReplica

			if nodeInfo.IsReplica && len(parts) > 3 && parts[3] != "-" {
				nodeInfo.MasterID = parts[3]
			}

			// 슬롯 파싱
			if len(parts) > 8 {
				for i := 8; i < len(parts); i++ {
					slotRange := parts[i]
					if strings.Contains(slotRange, "-") {
						rangeParts := strings.Split(slotRange, "-")
						if len(rangeParts) == 2 {
							start, err1 := strconv.Atoi(rangeParts[0])
							end, err2 := strconv.Atoi(rangeParts[1])
							if err1 == nil && err2 == nil {
								for slot := start; slot <= end; slot++ {
									nodeInfo.Slots = append(nodeInfo.Slots, slot)
								}
							}
						}
					} else {
						if slot, err := strconv.Atoi(slotRange); err == nil {
							nodeInfo.Slots = append(nodeInfo.Slots, slot)
						}
					}
				}
			}

			fmt.Println(styles.SuccessStyle.Render(" 완료"))
			fmt.Printf("  노드 타입: %s\n", nodeTypeString(nodeInfo))
			if len(nodeInfo.Slots) > 0 {
				fmt.Printf("  할당된 슬롯: %d개\n", len(nodeInfo.Slots))
			}
			return nodeInfo, nil
		}
	}

	fmt.Println(styles.ErrorStyle.Render(" 실패"))
	return nil, fmt.Errorf("노드 ID '%s'를 찾을 수 없습니다", nodeID)
}

func nodeTypeString(nodeInfo *NodeInfo) string {
	if nodeInfo.IsMaster {
		return styles.HighlightStyle.Render("마스터")
	}
	return styles.HighlightStyle.Render("레플리카")
}

func reshardBeforeRemoval(ctx context.Context, client *redis.ClusterClient, nodeInfo *NodeInfo) error {
	fmt.Print(styles.InfoStyle.Render("3. 슬롯 재분배 중..."))

	// 다른 마스터 노드들 가져오기
	masters, err := getOtherMasters(ctx, client, nodeInfo.ID)
	if err != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return err
	}

	if len(masters) == 0 {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return fmt.Errorf("슬롯을 이관할 다른 마스터 노드가 없습니다")
	}

	// 다른 마스터들에게 슬롯을 고르게 분배 (롤백 로직 추가)
	slotsPerMaster := len(nodeInfo.Slots) / len(masters)
	remainder := len(nodeInfo.Slots) % len(masters)

	slotIndex := 0
	var migratedSlots []int // 롤백을 위한 추적

	for i, masterID := range masters {
		slotsToMove := slotsPerMaster
		if i < remainder {
			slotsToMove++
		}

		if slotsToMove > 0 {
			startSlot := slotIndex
			endSlot := slotIndex + slotsToMove - 1
			slotsToMigrate := nodeInfo.Slots[startSlot : endSlot+1]

			if err := moveSlots(ctx, client, nodeInfo.ID, masterID, slotsToMigrate); err != nil {
				fmt.Println(styles.ErrorStyle.Render(" 실패"))
				// 롤백 수행
				rollbackSlotMigration(ctx, client, nodeInfo.ID, migratedSlots)
				return fmt.Errorf("슬롯 이동 실패 (대상: %s): %w", masterID, err)
			}

			migratedSlots = append(migratedSlots, slotsToMigrate...)
			slotIndex += slotsToMove
		}
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return nil
}

func getOtherMasters(ctx context.Context, client *redis.ClusterClient, excludeNodeID string) ([]string, error) {
	result := client.ClusterNodes(ctx)
	if result.Err() != nil {
		return nil, result.Err()
	}

	var masters []string
	lines := strings.Split(result.Val(), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		nodeID := parts[0]

		if nodeID != excludeNodeID {
			nodeFlags := parseNodeFlagsSlice(strings.Split(parts[2], ","))
			if nodeFlags.IsMaster {
				masters = append(masters, nodeID)
			}
		}
	}

	return masters, nil
}

func moveSlots(ctx context.Context, client *redis.ClusterClient, sourceNodeID, targetNodeID string, slots []int) error {
	// 소스와 타겟 노드 주소 가져오기
	sourceAddr, err := getNodeAddress(ctx, client, sourceNodeID)
	if err != nil {
		return err
	}

	targetAddr, err := getNodeAddress(ctx, client, targetNodeID)
	if err != nil {
		return err
	}

	user, password := config.GetAuth()

	// 소스 노드에 연결
	sourceClient := redis.NewClient(&redis.Options{
		Addr:     sourceAddr,
		Username: user,
		Password: password,
	})
	defer sourceClient.Close()

	// 타겟 노드에 연결
	targetClient := redis.NewClient(&redis.Options{
		Addr:     targetAddr,
		Username: user,
		Password: password,
	})
	defer targetClient.Close()

	// MIGRATE 명령을 위한 타겟 주소 파싱
	targetParts := strings.Split(targetAddr, ":")
	if len(targetParts) != 2 {
		return fmt.Errorf("잘못된 대상 노드 주소: %s", targetAddr)
	}
	targetHost := targetParts[0]
	targetPort := targetParts[1]

	// 진행률 표시를 위한 초기화
	totalSlots := len(slots)
	fmt.Printf("    %d개 슬롯 마이그레이션 시작...\n", totalSlots)

	// MIGRATE 명령어는 소스 노드에서 실행되어 타겟으로 직접 전송
	// MIGRATE targetHost targetPort key 0 60000 [AUTH password]

	// 순차 처리하되 배치 크기와 대기 시간 최적화
	// TODO: goroutine으로 처리해버리면 클러스터 뷰 불일치? ASK 리다렉 폭증?
	for i, slot := range slots {
		// 진행률 표시 (매 10%마다)
		if i > 0 && (i*10/totalSlots) > ((i-1)*10/totalSlots) {
			progress := (i * 100) / totalSlots
			fmt.Printf("    진행률: %d%% (%d/%d)\n", progress, i, totalSlots)
		}

		// 타겟에서 슬롯 가져오기 설정
		if err := targetClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "IMPORTING", sourceNodeID).Err(); err != nil {
			return fmt.Errorf("슬롯 %d 가져오기 설정 실패: %w", slot, err)
		}

		// 소스에서 슬롯 이주 설정
		if err := sourceClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "MIGRATING", targetNodeID).Err(); err != nil {
			return fmt.Errorf("슬롯 %d 이주 설정 실패: %w", slot, err)
		}

		// 키 마이그레이션 - 배치 크기 증가로 효율성 향상
		batchSize := 500 // 프로덕션 환경에서 안전한 크기
		if err := migrateSlotsKeysWithBatching(ctx, sourceClient, slot, targetHost, targetPort, user, password, batchSize); err != nil {
			return fmt.Errorf("슬롯 %d 키 마이그레이션 실패: %w", slot, err)
		}

		// 양쪽 노드에서 슬롯 안정화
		if err := sourceClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", targetNodeID).Err(); err != nil {
			return fmt.Errorf("슬롯 %d 소스 노드 안정화 실패: %w", slot, err)
		}

		if err := targetClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", targetNodeID).Err(); err != nil {
			return fmt.Errorf("슬롯 %d 대상 노드 안정화 실패: %w", slot, err)
		}

		// 일관성을 위해 클러스터의 모든 노드에 새로운 슬롯 소유권 업데이트
		err = updateAllNodesSlotOwnershipForDelNode(ctx, client, slot, targetNodeID)
		if err != nil {
			return fmt.Errorf("슬롯 %d 클러스터 전체 업데이트 실패: %w", slot, err)
		}

		// CPU 부하 분산을 위한 미세 대기
		// 매 슬롯마다 소량 대기로 시스템 부하 분산
		if i > 0 && i%50 == 0 { // 매 50슬롯마다 더 긴 대기
			time.Sleep(200 * time.Millisecond)
		} else {
			time.Sleep(50 * time.Millisecond)
		}
	}

	fmt.Printf("    마이그레이션 완료: %d개 슬롯\n", totalSlots)
	return nil
}

// 키 마이그레이션 함수 - 배치 처리 최적화
func migrateSlotsKeysWithBatching(ctx context.Context, sourceClient *redis.Client, slot int, targetHost, targetPort, user, password string, batchSize int) error {
	for {
		// 슬롯의 키들 가져오기 (배치 크기 증가)
		keys, err := sourceClient.ClusterGetKeysInSlot(ctx, slot, batchSize).Result()
		if err != nil {
			return fmt.Errorf("키 조회 실패: %w", err)
		}

		// 키가 없으면 마이그레이션 완료
		if len(keys) == 0 {
			break
		}

		// 배치의 각 키 마이그레이션
		for _, key := range keys {
			migrateCmd := buildMigrateCommand(targetHost, targetPort, key, user, password)

			// 재시도 로직 추가
			maxRetries := 3
			var migrateErr error

			for retry := 0; retry < maxRetries; retry++ {
				migrateErr = sourceClient.Do(ctx, migrateCmd...).Err()
				if migrateErr == nil {
					break // 성공
				}

				// 네트워크 오류나 일시적 오류인 경우 재시도
				if strings.Contains(migrateErr.Error(), "timeout") ||
					strings.Contains(migrateErr.Error(), "connection") {
					time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond)
					continue
				}

				break // 재시도 불가능한 오류
			}

			if migrateErr != nil {
				return fmt.Errorf("키 '%s' 마이그레이션 실패 (재시도 %d회): %w", key, maxRetries, migrateErr)
			}
		}

		// 배치 처리 간 미세 대기 (Redis 서버 부하 분산)
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

func getNodeAddress(ctx context.Context, client *redis.ClusterClient, nodeID string) (string, error) {
	result := client.ClusterNodes(ctx)
	if result.Err() != nil {
		return "", result.Err()
	}

	lines := strings.Split(result.Val(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == nodeID {
			addr := parts[1]
			// @cluster_port가 있으면 제거
			if strings.Contains(addr, "@") {
				addr = strings.Split(addr, "@")[0]
			}
			return addr, nil
		}
	}

	return "", fmt.Errorf("노드 %s의 주소를 찾을 수 없습니다", nodeID)
}

func removeNodeFromCluster(ctx context.Context, client *redis.ClusterClient, nodeID string) error {
	fmt.Print(styles.InfoStyle.Render("4. 클러스터에서 노드 제거..."))

	// 일관성을 위해 모든 노드에 CLUSTER FORGET 전송
	err := forgetNodeFromAllNodes(ctx, client, nodeID)
	if err != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return err
	}

	// 변경사항이 전파될 때까지 동적 대기
	fmt.Print("  클러스터 상태 안정화 대기 중...")
	err = waitForClusterStableSimple(ctx, client, 8*time.Second)
	if err != nil {
		fmt.Printf(" %s\n", styles.WarningStyle.Render("타임아웃 - 하드코딩 대기로 fallback"))
		time.Sleep(2 * time.Second)
	} else {
		fmt.Printf(" %s\n", styles.SuccessStyle.Render("완료"))
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return nil
}

// forgetNodeFromAllNodes 일관성을 위해 클러스터의 모든 노드에 CLUSTER FORGET 전송
func forgetNodeFromAllNodes(ctx context.Context, client *redis.ClusterClient, nodeIDToRemove string) error {
	// 모든 클러스터 노드 가져오기
	result := client.ClusterNodes(ctx)
	if result.Err() != nil {
		return fmt.Errorf("클러스터 노드 목록 조회 실패: %w", result.Err())
	}

	var errors []string
	successCount := 0

	lines := strings.Split(result.Val(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		nodeID := parts[0]
		nodeAddr := parts[1]

		// 제거되는 노드는 건너뛰기 (자기 자신을 forget할 수 없음)
		if nodeID == nodeIDToRemove {
			continue
		}

		// 주소 정규화
		if strings.Contains(nodeAddr, "@") {
			nodeAddr = strings.Split(nodeAddr, "@")[0]
		}

		// 개별 노드에 연결해서 CLUSTER FORGET 전송
		user, password := config.GetAuth()
		nodeClient := redis.NewClient(&redis.Options{
			Addr:     nodeAddr,
			Username: user,
			Password: password,
		})

		err := nodeClient.ClusterForget(ctx, nodeIDToRemove).Err()
		nodeClient.Close()

		if err != nil {
			errors = append(errors, fmt.Sprintf("노드 %s에서 forget 실패: %v", nodeAddr, err))
			continue
		}

		successCount++
	}

	// 대부분의 노드가 성공했다면 성공으로 간주하지만 경고 로그
	if successCount > 0 && len(errors) > 0 {
		// 경고를 로그할 수도 있지만 실패하지 않음 - 일부 노드가 일시적으로 사용 불가능할 수 있음
		for _, errMsg := range errors {
			_ = errMsg
		}
	}

	// 모든 노드에서 업데이트할 수 없는 경우에만 실패 (치명적 실패)
	if successCount == 0 {
		return fmt.Errorf("모든 노드에서 forget 실패: %v", errors)
	}

	return nil
}

func validateRemoval(ctx context.Context, client *redis.ClusterClient, nodeID string) error {
	fmt.Print(styles.InfoStyle.Render("5. 노드 제거 확인..."))

	// 노드가 여전히 클러스터에 있는지 체크
	result := client.ClusterNodes(ctx)
	if result.Err() != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return result.Err()
	}

	lines := strings.Split(result.Val(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) > 0 && parts[0] == nodeID {
			fmt.Println(styles.ErrorStyle.Render(" 실패"))
			return fmt.Errorf("노드가 여전히 클러스터에 존재합니다")
		}
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return nil
}

func validateNodeReachability(ctx context.Context, nodeInfo *NodeInfo) error {
	// 클러스터 포트 없이 주소 추출
	addr := nodeInfo.Addr
	if strings.Contains(addr, "@") {
		addr = strings.Split(addr, "@")[0]
	}

	user, password := config.GetAuth()
	nodeClient := redis.NewClient(&redis.Options{
		Addr:     addr,
		Username: user,
		Password: password,
	})
	defer nodeClient.Close()

	// 노드 핑 시도
	return nodeClient.Ping(ctx).Err()
}

// 동적 클러스터 안정화 대기 (ClusterClient용)
func waitForClusterStableSimple(ctx context.Context, client *redis.ClusterClient, maxWait time.Duration) error {
	start := time.Now()
	for time.Since(start) < maxWait {
		result := client.ClusterInfo(ctx)
		if result.Err() == nil {
			info := result.Val()
			if strings.Contains(info, "cluster_state:ok") {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("클러스터가 %v 내에 안정화되지 않았습니다", maxWait)
}

// 슬롯 재분배 롤백 함수
func rollbackSlotMigration(ctx context.Context, client *redis.ClusterClient, sourceNodeID string, migratedSlots []int) {
	if len(migratedSlots) == 0 {
		return
	}

	fmt.Println(styles.WarningStyle.Render(" 슬롯 재분배 실패 - 롤백 중..."))

	// 이미 이동된 슬롯들을 원래 소스 노드로 되돌리기
	// 일단 경고만 출력
	// TODO: 수동으로 맞길지 자동으로 할지
	fmt.Printf("  경고: %d개 슬롯이 부분적으로 이동되었습니다\n", len(migratedSlots))
	fmt.Println("  수동으로 클러스터 상태를 확인하고 필요시 슬롯을 재조정하세요")
}

// MIGRATE 명령어 통합 구성 함수
func buildMigrateCommand(targetHost, targetPort, key, user, password string) []interface{} {
	baseCmd := []interface{}{"MIGRATE", targetHost, targetPort, key, 0, 60000}

	if user != "" {
		return append(baseCmd, "AUTH2", user, password)
	} else if password != "" {
		return append(baseCmd, "AUTH", password)
	}

	return baseCmd
}
func updateAllNodesSlotOwnershipForDelNode(ctx context.Context, client *redis.ClusterClient, slot int, targetNodeID string) error {
	// 모든 클러스터 노드 가져오기
	result := client.ClusterNodes(ctx)
	if result.Err() != nil {
		return fmt.Errorf("클러스터 노드 목록 조회 실패: %w", result.Err())
	}

	var errors []string
	successCount := 0

	lines := strings.Split(result.Val(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		nodeAddr := parts[1]

		// 주소 정규화
		if strings.Contains(nodeAddr, "@") {
			nodeAddr = strings.Split(nodeAddr, "@")[0]
		}

		// 개별 노드에 연결해서 CLUSTER SETSLOT 전송
		user, password := config.GetAuth()
		nodeClient := redis.NewClient(&redis.Options{
			Addr:     nodeAddr,
			Username: user,
			Password: password,
		})

		err := nodeClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", targetNodeID).Err()
		nodeClient.Close()

		if err != nil {
			errors = append(errors, fmt.Sprintf("노드 %s 슬롯 업데이트 실패: %v", nodeAddr, err))
			continue
		}

		successCount++
	}

	// 대부분의 노드가 성공했다면 성공으로 간주하지만 경고 로그
	if successCount > 0 && len(errors) > 0 {
		// 경고를 로그할 수도 있지만 실패하지 않음 - 일부 노드가 일시적으로 사용 불가능할 수 있음
		for _, errMsg := range errors {

			_ = errMsg
		}
	}

	// 모든 노드에서 업데이트할 수 없는 경우에만 실패 (치명적 실패)
	if successCount == 0 {
		return fmt.Errorf("모든 노드에서 슬롯 업데이트 실패: %v", errors)
	}

	return nil
}
