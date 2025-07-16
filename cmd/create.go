package cmd

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"redisctl/internal/config"
	"redisctl/internal/redis"
	"redisctl/internal/styles"

	"github.com/spf13/cobra"
)

// NewCreateCommand create 명령어
func NewCreateCommand() *cobra.Command {
	var replicas int

	cmd := &cobra.Command{
		Use:   "create [--replicas N] ip1:port1 ... ipN:portN",
		Short: "< Redis 클러스터를 생성합니다",
		Long: styles.TitleStyle.Render("[T] Redis 클러스터 생성") + "\n\n" +
			styles.DescStyle.Render("지정된 노드들로부터 Redis 클러스터를 초기화합니다.") + "\n" +
			styles.DescStyle.Render("최소 노드 요구사항을 충족해야 하며, 마스터 노드들 간에 슬롯을 균등하게 분배합니다."),
		Example: `  # 3개 노드로 클러스터 생성 (복제본 없음)
  redisctl create localhost:7001 localhost:7002 localhost:7003

  # 6개 노드로 클러스터 생성 (각 마스터당 복제본 1개)
  redisctl create --replicas 1 localhost:7001 localhost:7002 localhost:7003 localhost:7004 localhost:7005 localhost:7006`,
		Args: cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.ValidateAuth(); err != nil {
				return err
			}

			return runCreateCluster(args, replicas)
		},
	}

	cmd.Flags().IntVar(&replicas, "replicas", 0, "각 마스터당 복제본 수 (기본값: 0)")

	return cmd
}

func runCreateCluster(nodes []string, replicas int) error {
	fmt.Println(styles.InfoStyle.Render("클러스터 생성 시작..."))
	fmt.Printf("노드 수: %d, 복제본: %d\n", len(nodes), replicas)

	// 입력 검증 개선
	if err := validateClusterInput(nodes, replicas); err != nil {
		return err
	}

	// 최소 노드 수 검증
	minNodes := calculateMinNodes(replicas)
	if len(nodes) < minNodes {
		return fmt.Errorf("최소 %d개의 노드가 필요합니다 (현재: %d개)", minNodes, len(nodes))
	}

	user, password := config.GetAuth()
	cm := redis.NewClusterManager(user, password)
	defer cm.Close()

	ctx := context.Background()

	// 모든 노드가 도달 가능하고 클러스터 모드 아님을 확인 (병렬 처리)
	fmt.Println(styles.InfoStyle.Render("1단계: 노드 연결 확인 중..."))

	type nodeCheckResult struct {
		node    string
		index   int
		err     error
		success bool
		message string
	}

	results := make(chan nodeCheckResult, len(nodes))
	var wg sync.WaitGroup

	// 병렬로 노드 연결 확인
	for i, node := range nodes {
		wg.Add(1)
		go func(index int, nodeAddr string) {
			defer wg.Done()

			_, err := cm.Connect(nodeAddr)
			if err != nil {
				results <- nodeCheckResult{
					node:    nodeAddr,
					index:   index,
					err:     fmt.Errorf("노드 %s 연결 실패: %w", nodeAddr, err),
					success: false,
					message: "연결 실패",
				}
				return
			}

			// 이미 클러스터에 참여 중인지 체크
			info, err := cm.GetClusterInfo(nodeAddr)
			if err == nil && info["cluster_state"] != "fail" {
				results <- nodeCheckResult{
					node:    nodeAddr,
					index:   index,
					err:     fmt.Errorf("노드 %s는 이미 클러스터에 참여 중입니다", nodeAddr),
					success: false,
					message: "이미 클러스터에 참여 중",
				}
				return
			}

			results <- nodeCheckResult{
				node:    nodeAddr,
				index:   index,
				err:     nil,
				success: true,
				message: "연결 성공",
			}
		}(i, node)
	}

	// 고루틴 완료 대기
	go func() {
		wg.Wait()
		close(results)
	}()

	// 결과 수집 및 정렬
	nodeResults := make([]nodeCheckResult, len(nodes))
	for result := range results {
		nodeResults[result.index] = result
	}

	// 순서대로 결과 출력
	for i, result := range nodeResults {
		fmt.Printf("  [%d/%d] %s 확인 중...", i+1, len(nodes), result.node)
		if result.success {
			fmt.Printf(" %s\n", styles.RenderSuccess(result.message))
		} else {
			if result.message == "이미 클러스터에 참여 중" {
				fmt.Printf(" %s\n", styles.RenderWarning(result.message))
			} else {
				fmt.Printf(" %s\n", styles.RenderError(result.message))
			}
		}

		if result.err != nil {
			return result.err
		}
	}

	// 마스터와 복제본 계산
	masters, replicaMap, err := calculateClusterLayout(nodes, replicas)
	if err != nil {
		return fmt.Errorf("클러스터 레이아웃 계산 실패: %w", err)
	}

	fmt.Println(styles.InfoStyle.Render("2단계: 클러스터 레이아웃 계획"))
	fmt.Printf("  마스터 노드: %d개\n", len(masters))
	fmt.Printf("  복제본: %d개\n", len(nodes)-len(masters))

	// 모든 노드들 만나게 하기
	fmt.Println(styles.InfoStyle.Render("3단계: 노드 간 핸드셰이크 수행 중..."))

	firstNode := nodes[0]
	firstClient, _ := cm.Connect(firstNode)

	for i, node := range nodes[1:] {
		fmt.Printf("  [%d/%d] %s와 핸드셰이크...", i+1, len(nodes)-1, node)

		host, port, err := parseNodeAddress(node)
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderError("주소 파싱 실패"))
			return err
		}

		// CLUSTER MEET 명령어
		err = firstClient.ClusterMeet(ctx, host, port).Err()
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderError("핸드셰이크 실패"))
			return fmt.Errorf("CLUSTER MEET 실패 (%s): %w", node, err)
		}

		fmt.Printf(" %s\n", styles.RenderSuccess("완료"))
	}

	// 슬롯 -> 마스터 (성능 개선: 에러 시 롤백 추가)
	fmt.Println(styles.InfoStyle.Render("4단계: 마스터 노드에 슬롯 할당 중..."))

	slotsPerMaster := 16384 / len(masters)
	remainder := 16384 % len(masters)

	currentSlot := 0
	assignedMasters := []string{} // 롤백을 위한 추적

	for i, masterNode := range masters {
		client, _ := cm.Connect(masterNode)

		slots := slotsPerMaster
		if i < remainder {
			slots++
		}

		endSlot := currentSlot + slots - 1
		fmt.Printf("  %s: 슬롯 %d-%d (%d개)...", masterNode, currentSlot, endSlot, slots)

		// 슬롯 배열 준비 - 더 안전한 방식
		slotArgs := make([]int, 0, slots)
		for slot := currentSlot; slot <= endSlot; slot++ {
			slotArgs = append(slotArgs, slot)
		}

		// CLUSTER ADDSLOTS 명령어 (에러 시 롤백)
		err := client.ClusterAddSlots(ctx, slotArgs...).Err()
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderError("슬롯 할당 실패"))
			// 부분적으로 할당된 슬롯들을 먼저 롤백
			rollbackPartialSlotAssignment(cm, assignedMasters)
			rollbackClusterCreation(cm, nodes)
			return fmt.Errorf("슬롯 할당 실패 (%s): %w", masterNode, err)
		}

		assignedMasters = append(assignedMasters, masterNode)
		fmt.Printf(" %s\n", styles.RenderSuccess("완료"))
		currentSlot += slots
	}

	// 복제본 설정 (롤백 로직 추가)
	if len(replicaMap) > 0 {
		fmt.Println(styles.InfoStyle.Render("5단계: 복제본 설정 중..."))

		// 클러스터 상태 안정화 대기 (성능 개선: 동적 대기)
		fmt.Print("  클러스터 안정화 대기 중...")
		err := waitForClusterStable(cm, firstNode, 10*time.Second)
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderWarning("타임아웃"))
			// 하드코딩 대기로 fallback
			time.Sleep(2 * time.Second)
		} else {
			fmt.Printf(" %s\n", styles.RenderSuccess("완료"))
		}

		for replicaNode, masterNode := range replicaMap {
			fmt.Printf("  %s -> %s 복제 설정...", replicaNode, masterNode)

			// 마스터 노드 ID 가져오기
			masterClient, _ := cm.Connect(masterNode)
			masterInfo, err := masterClient.ClusterMyID(ctx).Result()
			if err != nil {
				fmt.Printf(" %s\n", styles.RenderError("마스터 ID 조회 실패"))
				rollbackClusterCreation(cm, nodes) // 롤백 추가
				return fmt.Errorf("마스터 ID 조회 실패 (%s): %w", masterNode, err)
			}

			// 복제본 설정
			replicaClient, _ := cm.Connect(replicaNode)
			err = replicaClient.ClusterReplicate(ctx, masterInfo).Err()
			if err != nil {
				fmt.Printf(" %s\n", styles.RenderError("복제 설정 실패"))
				rollbackClusterCreation(cm, nodes) // 롤백 추가
				return fmt.Errorf("복제 설정 실패 (%s): %w", replicaNode, err)
			}

			fmt.Printf(" %s\n", styles.RenderSuccess("완료"))
		}
	}

	// 클러스터 검증 (더 상세한 검증)
	fmt.Println(styles.InfoStyle.Render("6단계: 클러스터 상태 확인 중..."))

	// 동적 안정화 대기
	fmt.Print("  클러스터 최종 안정화 대기 중...")
	err = waitForClusterStable(cm, firstNode, 15*time.Second)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderWarning("타임아웃 - 하드코딩 대기로 fallback"))
		time.Sleep(3 * time.Second)
	} else {
		fmt.Printf(" %s\n", styles.RenderSuccess("완료"))
	}

	// 클러스터 상태 검증
	fmt.Print("  클러스터 상태 검증 중...")
	info, err := cm.GetClusterInfo(firstNode)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderError("정보 조회 실패"))
		return fmt.Errorf("클러스터 정보 조회 실패: %w", err)
	}

	if info["cluster_state"] != "ok" {
		fmt.Printf(" %s\n", styles.RenderError("비정상 상태"))
		return fmt.Errorf("클러스터 상태가 비정상입니다: %s", info["cluster_state"])
	}
	fmt.Printf(" %s\n", styles.RenderSuccess("OK"))

	// 슬롯 커버리지 검증
	fmt.Print("  슬롯 커버리지 검증 중...")
	slotsCovered, err := verifySlotCoverage(cm, firstNode)
	if err != nil {
		fmt.Printf(" %s\n", styles.RenderWarning("검증 실패"))
		// 경고만 출력하고 계속 진행
	} else if slotsCovered != 16384 {
		fmt.Printf(" %s\n", styles.RenderWarning(fmt.Sprintf("부분 커버리지: %d/16384", slotsCovered)))
	} else {
		fmt.Printf(" %s\n", styles.RenderSuccess("완전 커버리지"))
	}

	// Success message
	fmt.Println()
	fmt.Println(styles.RenderSuccess("Redis 클러스터가 성공적으로 생성되었습니다!"))
	fmt.Println()
	fmt.Println(styles.BoxStyle.Render(
		styles.SubtitleStyle.Render("클러스터 정보") + "\n" +
			fmt.Sprintf("• 총 노드: %d개\n", len(nodes)) +
			fmt.Sprintf("• 마스터: %d개\n", len(masters)) +
			fmt.Sprintf("• 복제본: %d개\n", len(nodes)-len(masters)) +
			fmt.Sprintf("• 상태: %s", info["cluster_state"]),
	))

	// 선택사항: 클러스터 복원력 정보 표시
	resilienceInfo := analyzeClusterResilience(len(masters), len(nodes)-len(masters), replicas)
	fmt.Println()
	fmt.Println(styles.BoxStyle.Render(
		styles.SubtitleStyle.Render("클러스터 내결함성 분석") + "\n" +
			resilienceInfo,
	))

	return nil
}

// analyzeClusterResilience 하드웨어 장애로부터 클러스터 복구 능력 분석
func analyzeClusterResilience(masterCount, replicaCount, replicasPerMaster int) string {
	var analysis []string

	// 기본 복원력 분석
	if replicaCount == 0 {
		analysis = append(analysis, "! 복제본이 없어 마스터 노드 장애 시 데이터 손실 위험이 있습니다")
		analysis = append(analysis, "내결함성: 낮음 (마스터 노드 장애 시 해당 슬롯 데이터 손실)")
	} else {
		maxFailures := replicasPerMaster
		if replicasPerMaster > 0 {
			analysis = append(analysis, fmt.Sprintf("각 마스터당 %d개의 복제본으로 내결함성이 확보되었습니다", replicasPerMaster))
			analysis = append(analysis, fmt.Sprintf("내결함성: 각 마스터 그룹에서 최대 %d개 노드 동시 장애까지 복구 가능", maxFailures))
		} else {
			// 복제본이 불균등하게 분배된 경우
			avgReplicas := float64(replicaCount) / float64(masterCount)
			analysis = append(analysis, fmt.Sprintf("평균 마스터당 %.1f개의 복제본이 있습니다", avgReplicas))
			analysis = append(analysis, "내결함성: 부분적 (일부 마스터 그룹의 복제본 수가 다를 수 있음)")
		}
	}

	// 클러스터 전체 복원력
	if masterCount >= 3 {
		analysis = append(analysis, "최소 마스터 수(3개) 요구사항을 만족합니다")

		// 클러스터 생존 가능성 계산
		if replicaCount > 0 {
			// 최악의 경우, 마스터들을 무작위로 잃는다면
			minSurvivableFaults := 1
			if replicasPerMaster > 0 {
				minSurvivableFaults = replicasPerMaster
			}
			analysis = append(analysis, fmt.Sprintf("권장사항: 최소 %d개 노드 동시 장애까지 서비스 연속성 보장", minSurvivableFaults))
		}
	}

	// 추가 권장사항
	if replicaCount == 0 {
		analysis = append(analysis, "권장사항: 고가용성을 위해 --replicas 1 이상 설정을 고려하세요")
	}

	totalNodes := masterCount + replicaCount
	if totalNodes < 6 {
		analysis = append(analysis, "권장사항: 프로덕션 환경에서는 6개 이상의 노드 구성을 권장합니다")
	}

	result := ""
	for i, item := range analysis {
		if i > 0 {
			result += "\n"
		}
		result += "• " + item
	}
	return result
}

func calculateMinNodes(replicas int) int {
	if replicas == 0 {
		return 3 // 클러스터를 위한 최소 3개 마스터
	}
	return 3 * (replicas + 1) // 3개 마스터 + 마스터당 복제본
}

// validateClusterInput performs comprehensive input validation
func validateClusterInput(nodes []string, replicas int) error {
	// 복제본 수 검증
	if replicas < 0 {
		return fmt.Errorf("복제본 수는 0 이상이어야 합니다")
	}

	// 중복 노드 검증
	nodeSet := make(map[string]bool)
	for _, node := range nodes {
		if nodeSet[node] {
			return fmt.Errorf("중복된 노드가 있습니다: %s", node)
		}
		nodeSet[node] = true
		// 노드 주소 형식 검증
		host, portStr, err := parseNodeAddress(node)
		if err != nil {
			return fmt.Errorf("잘못된 노드 주소 형식 (%s): %w", node, err)
		}

		// 포트 범위 검증
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("잘못된 포트 번호 형식 (%s): %s", node, portStr)
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("잘못된 포트 번호 (%s): %d는 1-65535 범위를 벗어났습니다", node, port)
		}

		// IP 주소 기본 검증 (localhost 허용)
		if host == "" {
			return fmt.Errorf("빈 호스트 주소입니다: %s", node)
		}

		// 더 정교한 호스트 주소 검증
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			// IP 주소 파싱 시도
			ip := net.ParseIP(host)
			if ip == nil {
				// IP가 아니라면 호스트명 검증 (기본적인 DNS 형식 체크)
				if !isValidHostname(host) {
					return fmt.Errorf("잘못된 호스트 주소 형식 (%s): %s", node, host)
				}
			}
		}
	}

	return nil
}

// 개선된 클러스터 레이아웃 계산
func calculateClusterLayout(nodes []string, replicas int) ([]string, map[string]string, error) {
	totalNodes := len(nodes)

	// 마스터 수 계산 - 개선된 로직
	mastersCount := totalNodes / (replicas + 1)
	if mastersCount < 3 {
		if totalNodes < 3 {
			return nil, nil, fmt.Errorf("클러스터를 위해 최소 3개의 노드가 필요합니다")
		}
		// 노드가 부족하면 복제본 수를 줄여서라도 최소 3개 마스터 확보
		mastersCount = 3
	}

	// 복제본 분배 가능성 검증
	replicaNodes := totalNodes - mastersCount
	if replicas > 0 && replicaNodes < mastersCount {
		return nil, nil, fmt.Errorf(
			"요청된 복제본 수(%d)로는 모든 마스터에 균등한 복제본 배치가 불가능합니다. "+
				"마스터 %d개에 복제본 %d개 (부족: %d개)",
			replicas, mastersCount, replicaNodes, mastersCount-replicaNodes)
	}

	masters := nodes[:mastersCount]
	replicaNodesList := nodes[mastersCount:]

	// 복제본 매핑 생성 - 균등 분배
	replicaMap := make(map[string]string)
	masterIndex := 0

	for _, replicaNode := range replicaNodesList {
		replicaMap[replicaNode] = masters[masterIndex]
		masterIndex = (masterIndex + 1) % len(masters)
	}

	return masters, replicaMap, nil
}

// 호스트명 기본 검증 함수
func isValidHostname(hostname string) bool {
	if len(hostname) == 0 || len(hostname) > 253 {
		return false
	}

	// 기본적인 호스트명 형식 검증 (RFC 규격 완전 구현은 아니지만 실용적)
	for i, char := range hostname {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '-' && i > 0 && i < len(hostname)-1 {
			continue
		}
		if char == '.' {
			continue
		}
		return false
	}

	return true
}

// 슬롯 커버리지 검증 함수
func verifySlotCoverage(cm *redis.ClusterManager, node string) (int, error) {
	client, err := cm.Connect(node)
	if err != nil {
		return 0, err
	}

	ctx := context.Background()
	// CLUSTER SLOTS 명령어로 할당된 슬롯 확인
	result, err := client.ClusterSlots(ctx).Result()
	if err != nil {
		return 0, err
	}

	coveredSlots := 0
	for _, slot := range result {
		start := slot.Start
		end := slot.End
		coveredSlots += int(end - start + 1)
	}

	return coveredSlots, nil
}

// 부분적으로 할당된 슬롯들 롤백
func rollbackPartialSlotAssignment(cm *redis.ClusterManager, assignedMasters []string) {
	if len(assignedMasters) == 0 {
		return
	}

	fmt.Println(styles.WarningStyle.Render("  부분 할당된 슬롯들 롤백 중..."))

	for _, masterNode := range assignedMasters {
		client, err := cm.Connect(masterNode)
		if err != nil {
			continue
		}

		// 해당 노드의 모든 슬롯 제거
		ctx := context.Background()
		err = client.Do(ctx, "CLUSTER", "RESET", "SOFT").Err()
		if err != nil {
			// SOFT 리셋 실패 시 HARD 리셋 시도
			client.Do(ctx, "CLUSTER", "RESET", "HARD").Err()
		}
	}
}

// 롤백 함수
func rollbackClusterCreation(cm *redis.ClusterManager, nodes []string) {
	fmt.Println(styles.WarningStyle.Render("클러스터 생성 실패 - 롤백 중..."))

	for i, node := range nodes {
		fmt.Printf("  [%d/%d] %s 초기화 중...", i+1, len(nodes), node)

		// 클러스터 매니저를 통해 연결
		nodeClient, err := cm.Connect(node)
		if err != nil {
			fmt.Printf(" %s\n", styles.RenderWarning("연결 실패"))
			continue
		}

		// 노드 리셋 시도
		err = nodeClient.Do(context.Background(), "CLUSTER", "RESET", "HARD").Err()

		if err != nil {
			fmt.Printf(" %s\n", styles.RenderWarning("실패"))
		} else {
			fmt.Printf(" %s\n", styles.RenderSuccess("완료"))
		}
	}
}
