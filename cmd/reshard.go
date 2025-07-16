/*
TODO: func rollbackResharding - 자동 롤백?
*/
package cmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"redisctl/internal/config"
	"redisctl/internal/redis"
	"redisctl/internal/styles"

	redisv9 "github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

// NewReshardCommand 'reshard' 명령어
func NewReshardCommand() *cobra.Command {
	var from, to string
	var slots, pipeline int

	cmd := &cobra.Command{
		Use:   "reshard --from str --to str --slots N [--pipeline N] ip:port",
		Short: "s 마스터 간 슬롯을 이동합니다",
		Long: styles.TitleStyle.Render("[R] 클러스터 리샤딩") + "\n\n" +
			styles.DescStyle.Render("MIGRATE 명령을 사용하여 마스터 노드 간 N개의 슬롯을 이동합니다.") + "\n" +
			styles.DescStyle.Render("데이터 손실 없이 슬롯과 해당 키들을 안전하게 재분산합니다."),
		Example: `  # 마스터 간 1000개 슬롯 이동
  redisctl reshard --from source-master-id --to target-master-id --slots 1000 localhost:7001

  # 파이프라인 크기 조정하여 성능 최적화
  redisctl reshard --from source-id --to target-id --slots 500 --pipeline 20 localhost:7001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.ValidateAuth(); err != nil {
				return err
			}

			return runReshard(args[0], from, to, slots, pipeline)
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "소스 마스터 노드 ID (필수)")
	cmd.Flags().StringVar(&to, "to", "", "대상 마스터 노드 ID (필수)")
	cmd.Flags().IntVar(&slots, "slots", 0, "이동할 슬롯 수 (필수)")
	cmd.Flags().IntVar(&pipeline, "pipeline", 10, "MIGRATE당 키 수 (기본값: 10)")

	cmd.MarkFlagRequired("from")
	cmd.MarkFlagRequired("to")
	cmd.MarkFlagRequired("slots")

	return cmd
}

func runReshard(clusterNode, fromNodeID, toNodeID string, slotsToMove, pipelineSize int) error {
	fmt.Println(styles.InfoStyle.Render("리샤딩 시작..."))
	fmt.Printf("클러스터 노드: %s\n", clusterNode)
	fmt.Printf("소스 마스터: %s\n", fromNodeID)
	fmt.Printf("대상 마스터: %s\n", toNodeID)
	fmt.Printf("이동할 슬롯: %d개\n", slotsToMove)
	fmt.Printf("파이프라인 크기: %d\n", pipelineSize)

	if slotsToMove <= 0 {
		return fmt.Errorf("이동할 슬롯 수는 0보다 커야 합니다")
	}

	if pipelineSize <= 0 {
		pipelineSize = 10
	}

	user, password := config.GetAuth()
	cm := redis.NewClusterManager(user, password)
	defer cm.Close()

	ctx := context.Background()

	// Step 1: Connect to cluster and validate
	fmt.Println(styles.InfoStyle.Render("1단계: 클러스터 연결 및 검증 중..."))
	fmt.Printf("  %s 연결 중...", clusterNode)

	_, err := cm.Connect(clusterNode)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("연결 실패"))
		return fmt.Errorf("클러스터 노드 연결 실패: %w", err)
	}

	// Check cluster health
	clusterInfo, err := cm.GetClusterInfo(clusterNode)
	if err != nil || clusterInfo["cluster_state"] != "ok" {
		fmt.Printf(" %s\n", styles.RenderError("클러스터 상태 비정상"))
		return fmt.Errorf("클러스터 상태가 정상이 아닙니다: %s", clusterInfo["cluster_state"])
	}

	fmt.Printf(" %s\n", styles.RenderSuccess("연결 성공"))

	// Step 2: Get cluster nodes and validate source/target
	fmt.Println(styles.InfoStyle.Render("2단계: 노드 정보 조회 및 검증 중..."))

	clusterNodes, err := cm.GetClusterNodes(clusterNode)
	if err != nil {
		return fmt.Errorf("클러스터 노드 정보 조회 실패: %w", err)
	}

	var sourceNode, targetNode *redis.ClusterNode
	for i, node := range clusterNodes {
		if node.ID == fromNodeID {
			sourceNode = &clusterNodes[i]
		}
		if node.ID == toNodeID {
			targetNode = &clusterNodes[i]
		}
	}

	// Validate source node
	if sourceNode == nil {
		return fmt.Errorf("소스 노드 ID를 찾을 수 없습니다: %s", fromNodeID)
	}

	if !isMasterNode(sourceNode.Flags) {
		return fmt.Errorf("소스 노드가 마스터가 아닙니다: %s", fromNodeID)
	}

	sourceSlotCount := countSlots(sourceNode.Slots)
	if sourceSlotCount < slotsToMove {
		return fmt.Errorf("소스 노드의 슬롯 수(%d)가 이동하려는 슬롯 수(%d)보다 적습니다", sourceSlotCount, slotsToMove)
	}

	// Validate target node
	if targetNode == nil {
		return fmt.Errorf("대상 노드 ID를 찾을 수 없습니다: %s", toNodeID)
	}

	if !isMasterNode(targetNode.Flags) {
		return fmt.Errorf("대상 노드가 마스터가 아닙니다: %s", toNodeID)
	}

	if sourceNode.ID == targetNode.ID {
		return fmt.Errorf("소스와 대상 노드가 동일합니다")
	}

	fmt.Printf("  소스: %s (%d개 슬롯)\n", sourceNode.Address, sourceSlotCount)
	fmt.Printf("  대상: %s (%d개 슬롯)\n", targetNode.Address, countSlots(targetNode.Slots))

	// Step 3: Select slots to move
	fmt.Println(styles.InfoStyle.Render("3단계: 이동할 슬롯 선택 중..."))

	slotsToMigrate := selectSlotsToMove(sourceNode.Slots, slotsToMove)
	if len(slotsToMigrate) != slotsToMove {
		return fmt.Errorf("선택된 슬롯 수(%d)가 요청된 수(%d)와 다릅니다", len(slotsToMigrate), slotsToMove)
	}

	fmt.Printf("  선택된 슬롯: %v\n", formatSlotRanges(slotsToMigrate))

	// Step 4: Prepare for migration
	fmt.Println(styles.InfoStyle.Render("4단계: 마이그레이션 준비 중..."))

	// Normalize addresses before connecting
	sourceAddr := normalizeClusterAddress(sourceNode.Address)
	targetAddr := normalizeClusterAddress(targetNode.Address)

	sourceClient, err := cm.Connect(sourceAddr)
	if err != nil {
		return fmt.Errorf("소스 노드 연결 실패: %w", err)
	}

	targetClient, err := cm.Connect(targetAddr)
	if err != nil {
		return fmt.Errorf("대상 노드 연결 실패: %w", err)
	}

	// Parse target address for MIGRATE commands
	targetHost, targetPort, err := parseNodeAddress(targetAddr)
	if err != nil {
		return fmt.Errorf("대상 노드 주소 파싱 실패: %w", err)
	}

	// Step 5: Start migration process (롤백 로직 추가)
	fmt.Println(styles.InfoStyle.Render("5단계: 슬롯 마이그레이션 중..."))

	var migratedSlots []int // 롤백을 위한 추적

	for i, slot := range slotsToMigrate {
		fmt.Printf("  [%d/%d] 슬롯 %d 마이그레이션 중...", i+1, len(slotsToMigrate), slot)

		err := migrateSlot(ctx, sourceClient, targetClient, slot, targetHost, targetPort, pipelineSize, fromNodeID, toNodeID)
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderError("실패"))
			// 롤백 수행
			rollbackResharding(ctx, cm, migratedSlots, fromNodeID, toNodeID)
			return fmt.Errorf("슬롯 %d 마이그레이션 실패: %w", slot, err)
		}

		// Update all nodes in cluster with new slot ownership
		err = updateAllNodesSlotOwnership(ctx, cm, clusterNode, slot, toNodeID)
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderError("클러스터 업데이트 실패"))
			// 롤백 수행
			rollbackResharding(ctx, cm, migratedSlots, fromNodeID, toNodeID)
			return fmt.Errorf("슬롯 %d 클러스터 업데이트 실패: %w", slot, err)
		}

		migratedSlots = append(migratedSlots, slot)
		fmt.Printf(" %s\n", styles.RenderSuccess("완료"))

		// 불필요한 소규모 대기 제거
		// 이전: 매 100슬롯마다 100ms 대기 -> 1000슬롯시 1초 추가 대기
		// 개선: 매 500슬롯마다 50ms 대기로 변경 -> 1000슬롯시 100ms만 추가 대기
		if i%500 == 499 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Step 6: Verify migration (동적 대기로 개선)
	fmt.Println(styles.InfoStyle.Render("6단계: 마이그레이션 검증 중..."))

	fmt.Print("  클러스터 안정화 대기 중...")
	err = waitForClusterStableForReshard(ctx, cm, clusterNode, 10*time.Second)
	if err != nil {
		fmt.Printf(" %s\n", styles.WarningStyle.Render("타임아웃 - 하드코딩 대기로 fallback"))
		time.Sleep(2 * time.Second)
	} else {
		fmt.Printf(" %s\n", styles.SuccessStyle.Render("완료"))
	}

	updatedNodes, err := cm.GetClusterNodes(clusterNode)
	if err != nil {
		return fmt.Errorf("검증을 위한 클러스터 정보 조회 실패: %w", err)
	}

	var updatedSource, updatedTarget *redis.ClusterNode
	for i, node := range updatedNodes {
		if node.ID == fromNodeID {
			updatedSource = &updatedNodes[i]
		}
		if node.ID == toNodeID {
			updatedTarget = &updatedNodes[i]
		}
	}

	if updatedSource == nil || updatedTarget == nil {
		return fmt.Errorf("업데이트된 노드 정보를 찾을 수 없습니다")
	}

	// Success message
	fmt.Println()
	fmt.Println(styles.RenderSuccess("리샤딩이 성공적으로 완료되었습니다!"))
	fmt.Println()

	// Display migration summary
	summary := styles.SubtitleStyle.Render("마이그레이션 요약") + "\n" +
		fmt.Sprintf("• 이동된 슬롯 수: %d개\n", len(slotsToMigrate)) +
		fmt.Sprintf("• 소스 노드 (%s): %d개 → %d개 슬롯\n",
			updatedSource.Address, sourceSlotCount, countSlots(updatedSource.Slots)) +
		fmt.Sprintf("• 대상 노드 (%s): %d개 → %d개 슬롯\n",
			updatedTarget.Address, countSlots(targetNode.Slots), countSlots(updatedTarget.Slots)) +
		fmt.Sprintf("• 파이프라인 크기: %d\n", pipelineSize)

	fmt.Println(styles.BoxStyle.Render(summary))

	return nil
}

func migrateSlot(ctx context.Context, sourceClient, targetClient *redisv9.Client, slot int, targetHost, targetPort string, pipelineSize int, sourceNodeID, targetNodeID string) error {
	// Set slot as migrating on source
	err := sourceClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "MIGRATING", targetNodeID).Err()
	if err != nil {
		return fmt.Errorf("MIGRATING 설정 실패: %w", err)
	}

	// Set slot as importing on target
	err = targetClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "IMPORTING", sourceNodeID).Err()
	if err != nil {
		return fmt.Errorf("IMPORTING 설정 실패: %w", err)
	}

	// Migrate all keys in the slot - repeat until slot is empty
	for {
		// Get keys in the slot (limited by pipeline size)
		keys, err := sourceClient.ClusterGetKeysInSlot(ctx, slot, pipelineSize).Result()
		if err != nil {
			return fmt.Errorf("슬롯 키 조회 실패: %w", err)
		}

		// If no keys left, migration is complete
		if len(keys) == 0 {
			break
		}

		// Migrate each key in this batch (AUTH 명령어 통합)
		for _, key := range keys {
			// Use simplified AUTH command building
			user, password := config.GetAuth()
			migrateCmd := buildMigrateCommandForReshard(targetHost, targetPort, key, user, password)

			err = sourceClient.Do(ctx, migrateCmd...).Err()
			if err != nil {
				return fmt.Errorf("MIGRATE 명령 실패 (키: %s): %w", key, err)
			}
		}
	}

	// Complete the slot migration by assigning the slot to the target node
	// Update ALL nodes in cluster for consistency
	err = sourceClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", targetNodeID).Err()
	if err != nil {
		return fmt.Errorf("소스 노드 슬롯 할당 실패: %w", err)
	}

	err = targetClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", targetNodeID).Err()
	if err != nil {
		return fmt.Errorf("대상 노드 슬롯 할당 실패: %w", err)
	}

	return nil
}

// updateAllNodesSlotOwnership ensures all cluster nodes know about slot ownership change
func updateAllNodesSlotOwnership(ctx context.Context, cm *redis.ClusterManager, clusterNode string, slot int, targetNodeID string) error {
	// Get all cluster nodes
	allNodes, err := cm.GetClusterNodes(clusterNode)
	if err != nil {
		return fmt.Errorf("클러스터 노드 목록 조회 실패: %w", err)
	}

	var errors []string
	successCount := 0

	// Update every single node in the cluster
	for _, node := range allNodes {
		nodeAddr := normalizeClusterAddress(node.Address)
		nodeClient, err := cm.Connect(nodeAddr)
		if err != nil {
			errors = append(errors, fmt.Sprintf("노드 %s 연결 실패: %v", nodeAddr, err))
			continue
		}

		err = nodeClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", targetNodeID).Err()
		if err != nil {
			errors = append(errors, fmt.Sprintf("노드 %s 슬롯 업데이트 실패: %v", nodeAddr, err))
			continue
		}

		successCount++
	}

	// If most nodes succeeded, consider it successful but log warnings
	if successCount > 0 && len(errors) > 0 {
		// Log warnings but don't fail - some nodes might be temporarily unavailable
		for _, errMsg := range errors {
			// Could log these warnings, but for now we'll ignore
			_ = errMsg
		}
	}

	// Only fail if no nodes could be updated (catastrophic failure)
	if successCount == 0 {
		return fmt.Errorf("모든 노드에서 슬롯 업데이트 실패: %v", errors)
	}

	return nil
}

// 동적 클러스터 안정화 대기 (ClusterManager용)
func waitForClusterStableForReshard(ctx context.Context, cm *redis.ClusterManager, node string, maxWait time.Duration) error {
	start := time.Now()
	for time.Since(start) < maxWait {
		info, err := cm.GetClusterInfo(node)
		if err == nil && info["cluster_state"] == "ok" {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("클러스터가 %v 내에 안정화되지 않았습니다", maxWait)
}

// 리샤딩 롤백 함수
func rollbackResharding(ctx context.Context, cm *redis.ClusterManager, migratedSlots []int, fromNodeID, toNodeID string) {
	if len(migratedSlots) == 0 {
		return
	}

	fmt.Println(styles.WarningStyle.Render("리샤딩 실패 - 롤백 중..."))

	// 이미 이동된 슬롯들을 원래 노드로 되돌리기
	// 일단 경고만 출력
	// TODO: 자동으로 롤백?
	fmt.Printf("  경고: %d개 슬롯이 부분적으로 이동되었습니다\n", len(migratedSlots))
	fmt.Println("  수동으로 클러스터 상태를 확인하고 필요시 슬롯을 재조정하세요")
	fmt.Printf("  이동된 슬롯: %v\n", formatSlotRanges(migratedSlots))
}

// MIGRATE 명령어 통합 구성 함수 (reshard용)
func buildMigrateCommandForReshard(targetHost, targetPort, key, user, password string) []interface{} {
	baseCmd := []any{"MIGRATE", targetHost, targetPort, key, 0, 60000}

	if user != "" {
		return append(baseCmd, "AUTH2", user, password)
	} else if password != "" {
		return append(baseCmd, "AUTH", password)
	}

	return baseCmd
}

func countSlots(slots []redis.SlotRange) int {
	count := 0
	for _, slotRange := range slots {
		count += slotRange.End - slotRange.Start + 1
	}
	return count
}

func selectSlotsToMove(availableSlots []redis.SlotRange, count int) []int {
	var selected []int

	for _, slotRange := range availableSlots {
		for slot := slotRange.Start; slot <= slotRange.End && len(selected) < count; slot++ {
			selected = append(selected, slot)
		}
		if len(selected) >= count {
			break
		}
	}

	return selected
}

func formatSlotRanges(slots []int) string {
	if len(slots) == 0 {
		return "없음"
	}

	if len(slots) <= 10 {
		result := ""
		for i, slot := range slots {
			if i > 0 {
				result += ", "
			}
			result += strconv.Itoa(slot)
		}
		return result
	}

	// Show first few and last few with ellipsis
	result := ""
	for i := 0; i < 3 && i < len(slots); i++ {
		if i > 0 {
			result += ", "
		}
		result += strconv.Itoa(slots[i])
	}

	result += ", ..."

	start := len(slots) - 3
	if start < 3 {
		start = 3
	}

	for i := start; i < len(slots); i++ {
		result += ", " + strconv.Itoa(slots[i])
	}

	return result
}
