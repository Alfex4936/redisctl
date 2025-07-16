/*
TODO: func getSampledKeyCount - 샘플링 수에 따른 벤치마크 + 키 개수 정확도?
*/

package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"

	"redisctl/internal/config"
	"redisctl/internal/styles"
)

// NewCheckCommand check 명령어
func NewCheckCommand() *cobra.Command {
	var verbose bool
	var raw bool
	var dbsize bool

	cmd := &cobra.Command{
		Use:   "check <cluster-node-ip:port>",
		Short: "^ Redis 클러스터 상태를 확인합니다",
		Long: styles.TitleStyle.Render("[S] Redis 클러스터 상태 확인") + "\n\n" +
			styles.DescStyle.Render("Redis 클러스터의 전반적인 상태를 확인하고 보고서를 생성합니다.") + "\n\n" +
			styles.DescStyle.Render("확인하는 항목들:") + "\n" +
			styles.DescStyle.Render("• 클러스터 연결 및 노드 상태") + "\n" +
			styles.DescStyle.Render("• 슬롯 분배 및 커버리지 (0-16383)") + "\n" +
			styles.DescStyle.Render("• 마스터-레플리카 관계") + "\n" +
			styles.DescStyle.Render("• 노드 간 일관성 검증") + "\n" +
			styles.DescStyle.Render("• 클러스터 성능 통계"),
		Example: `  # 클러스터 상태 확인
  redisctl check localhost:7001

  # 상세한 클러스터 보고서 생성  
  redisctl --password mypass check localhost:9001

  # 원시 노드 데이터 포함 상세 보고서
  redisctl check --verbose localhost:9001

  # 원시 cluster nodes 출력
  redisctl check --raw localhost:9001

  # 정확한 키 개수 확인 (느림)
  redisctl check --dbsize localhost:9001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.ValidateAuth(); err != nil {
				return err
			}
			return runCheckCluster(args[0], verbose, raw, dbsize)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "상세한 노드 정보 표시 (full ID, epoch, flags 등)")
	cmd.Flags().BoolVar(&raw, "raw", false, "원시 cluster nodes 출력 표시")
	cmd.Flags().BoolVar(&dbsize, "dbsize", false, "정확한 키 개수 확인 (느릴 수 있음, 기본값은 빠른 샘플링)")

	return cmd
}

type ClusterNode struct {
	ID          string
	Addr        string
	Flags       []string
	MasterID    string
	Slots       []int
	IsMaster    bool
	IsReplica   bool
	IsFail      bool
	IsHandshake bool
	Replicas    []string
}

type ClusterStatus struct {
	Nodes           []ClusterNode
	TotalSlots      int
	TotalKeys       int64
	Masters         int
	Replicas        int
	FailedNodes     int
	SlotsCovered    []bool // 16384 elements, true if slot is covered
	KnownNodesCount int
	ClusterSize     int
	CurrentEpoch    int64
	MyEpoch         int64
	ClusterState    string
	PreciseKeyCount bool // true if dbsize was used for accurate count
}

func runCheckCluster(clusterAddr string, verbose, raw, dbsize bool) error {
	fmt.Println(styles.InfoStyle.Render("[::] Redis 클러스터 상태 확인"))
	fmt.Printf("클러스터: %s\n", styles.HighlightStyle.Render(clusterAddr))
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
	if err := validateCheckConnectivity(ctx, client); err != nil {
		return fmt.Errorf("클러스터 연결 실패: %w", err)
	}

	// 클러스터 상태 전체적으로 가져오기
	status, err := getClusterStatus(ctx, client, dbsize)
	if err != nil {
		return fmt.Errorf("클러스터 상태 조회 실패: %w", err)
	}

	// 클러스터 상태가 'fail'인 경우 재확인 (일시적 상태 감지)
	if status.ClusterState == "fail" {
		fmt.Print(styles.WarningStyle.Render("  클러스터 상태 'fail' 감지 - 재확인 중..."))
		time.Sleep(2 * time.Second) // 2초 대기 후 재확인

		retryStatus, retryErr := getClusterStatusQuick(ctx, client)
		if retryErr == nil && retryStatus != "fail" {
			fmt.Printf(" %s\n", styles.SuccessStyle.Render("복구됨"))
			status.ClusterState = retryStatus
			fmt.Printf("  ℹ️ 일시적 상태 변화 감지됨 (fail → %s)\n", retryStatus)
		} else {
			fmt.Printf(" %s\n", styles.ErrorStyle.Render("여전히 fail"))
		}
		fmt.Println()
	}

	// raw 출력 요청시 원시 데이터 보여주기
	if raw {
		fmt.Println(styles.TitleStyle.Render("원시 클러스터 노드 데이터"))
		result := client.ClusterNodes(ctx)
		if result.Err() == nil {
			fmt.Println(result.Val())
		}
		fmt.Println()
	}

	// 리포트 생성해서 보여주기
	displayClusterReport(status, verbose)

	// 건강성 체크들 돌려보기
	healthIssues := runHealthChecks(status)

	// 모든 노드에서 클러스터 정보 일관성 체크
	consistencyIssues, err := checkClusterConsistency(ctx, client, status)
	if err != nil {
		fmt.Printf("클러스터 일관성 검사 실패: %v\n", err)
	} else {
		healthIssues = append(healthIssues, consistencyIssues...)
	}

	displayHealthReport(healthIssues)

	return nil
}

func validateCheckConnectivity(ctx context.Context, client *redis.ClusterClient) error {
	fmt.Print(styles.InfoStyle.Render("1. 클러스터 연결 확인..."))

	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return err
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return nil
}

func getClusterStatus(ctx context.Context, client *redis.ClusterClient, dbsize bool) (*ClusterStatus, error) {
	fmt.Print(styles.InfoStyle.Render("2. 클러스터 상태 수집..."))

	// 클러스터 노드들 정보 가져오기
	result := client.ClusterNodes(ctx)
	if result.Err() != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return nil, result.Err()
	}

	status := &ClusterStatus{
		SlotsCovered: make([]bool, 16384),
	}

	// 클러스터 노드 파싱
	lines := strings.Split(result.Val(), "\n")
	malformedLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		node, err := parseClusterNode(line)
		if err != nil {
			malformedLines++
			continue // Skip malformed lines
		}

		status.Nodes = append(status.Nodes, *node)

		// 카운터 업데이트
		if node.IsMaster {
			status.Masters++
		}
		if node.IsReplica {
			status.Replicas++
		}
		if node.IsFail {
			status.FailedNodes++
		}

		// 커버된 슬롯 표시
		for _, slot := range node.Slots {
			if slot >= 0 && slot < 16384 {
				status.SlotsCovered[slot] = true
			}
		}
	}

	// malformed 라인에 대한 경고
	if malformedLines > 0 {
		fmt.Printf("\n  경고: %d개의 잘못된 노드 라인이 무시되었습니다\n", malformedLines)
	}

	// 총 커버된 슬롯 수 계산
	for _, covered := range status.SlotsCovered {
		if covered {
			status.TotalSlots++
		}
	}

	// 추가 통계를 위한 클러스터 정보 가져오기
	infoResult := client.ClusterInfo(ctx)
	if infoResult.Err() == nil {
		infoLines := strings.Split(infoResult.Val(), "\n")
		for _, line := range infoLines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "cluster_known_nodes:") {
				if parts := strings.Split(line, ":"); len(parts) == 2 {
					if knownNodes, err := strconv.Atoi(parts[1]); err == nil {
						// 알려진 노드 수와 실제 노드가 다르면 클러스터 불일치일 수 있음
						if knownNodes != len(status.Nodes) {
							status.KnownNodesCount = knownNodes
						}
					}
				}
			} else if strings.HasPrefix(line, "cluster_size:") {
				if parts := strings.Split(line, ":"); len(parts) == 2 {
					if clusterSize, err := strconv.Atoi(parts[1]); err == nil {
						status.ClusterSize = clusterSize
					}
				}
			} else if strings.HasPrefix(line, "cluster_current_epoch:") {
				if parts := strings.Split(line, ":"); len(parts) == 2 {
					if epoch, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
						status.CurrentEpoch = epoch
					}
				}
			} else if strings.HasPrefix(line, "cluster_my_epoch:") {
				if parts := strings.Split(line, ":"); len(parts) == 2 {
					if myEpoch, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
						status.MyEpoch = myEpoch
					}
				}
			} else if strings.HasPrefix(line, "cluster_state:") {
				if parts := strings.Split(line, ":"); len(parts) == 2 {
					status.ClusterState = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// 키 개수 추정치 구하기
	status.TotalKeys = getEstimatedKeyCount(ctx, client, dbsize)
	status.PreciseKeyCount = dbsize

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return status, nil
}

func parseClusterNode(line string) (*ClusterNode, error) {
	parts := strings.Fields(line)
	if len(parts) < 8 {
		return nil, fmt.Errorf("malformed node line")
	}

	node := &ClusterNode{
		ID:   parts[0],
		Addr: parts[1],
	}

	// 플래그 파싱 using unified utility
	flags := strings.Split(parts[2], ",")
	node.Flags = flags
	nodeFlags := parseNodeFlagsSlice(flags)
	node.IsMaster = nodeFlags.IsMaster
	node.IsReplica = nodeFlags.IsReplica
	node.IsFail = nodeFlags.IsFail
	node.IsHandshake = nodeFlags.IsHandshake

	// 레플리카를 위한 마스터 ID 파싱
	if len(parts) > 3 && parts[3] != "-" {
		node.MasterID = parts[3]
	}

	// 슬롯 파싱 (인덱스 8부터 시작)
	if len(parts) > 8 {
		for i := 8; i < len(parts); i++ {
			slotRange := parts[i]

			// 임포팅/마이그레이팅 상태 슬롯 처리 (예: [1234->-node_id] [<-1234-node_id])
			if strings.HasPrefix(slotRange, "[") {
				continue // 임시 상태 슬롯은 건너뛰기
			}

			if strings.Contains(slotRange, "-") {
				// Range like "0-5460"
				rangeParts := strings.Split(slotRange, "-")
				if len(rangeParts) == 2 {
					start, err1 := strconv.Atoi(rangeParts[0])
					end, err2 := strconv.Atoi(rangeParts[1])
					if err1 == nil && err2 == nil && start >= 0 && end < 16384 && start <= end {
						for slot := start; slot <= end; slot++ {
							node.Slots = append(node.Slots, slot)
						}
					}
				}
			} else {
				// Single slot
				if slot, err := strconv.Atoi(slotRange); err == nil && slot >= 0 && slot < 16384 {
					node.Slots = append(node.Slots, slot)
				}
			}
		}
	}

	// 역할 재검증: 더 정확한 마스터/복제본 판단
	// 복제본은 마스터 ID가 있어야 하고, 마스터는 슬롯을 가지거나 master 플래그가 있어야 함
	if node.MasterID != "" && node.MasterID != "-" {
		// 마스터 ID가 있으면 복제본
		node.IsReplica = true
		node.IsMaster = false
	} else if len(node.Slots) > 0 {
		// 슬롯을 가지고 있으면 마스터
		node.IsMaster = true
		node.IsReplica = false
	} else {
		// 플래그 기반으로만 판단 (기존 로직 유지)
		// 이미 위에서 플래그 파싱됨
	}

	return node, nil
}

func getEstimatedKeyCount(ctx context.Context, client *redis.ClusterClient, dbsize bool) int64 {
	if dbsize {
		// 정확한 키 개수: 모든 슬롯에서 키 개수 합산
		return getPreciseKeyCount(ctx, client)
	}

	// 기본 샘플링 방식 (빠름)
	return getSampledKeyCount(ctx, client)
}

func getPreciseKeyCount(ctx context.Context, client *redis.ClusterClient) int64 {
	var totalKeys int64

	// 모든 16384 슬롯에서 키 개수 확인 (느릴 수 있음)
	for slot := 0; slot < 16384; slot++ {
		result := client.ClusterCountKeysInSlot(ctx, slot)
		if result.Err() == nil {
			totalKeys += result.Val()
		}
	}

	return totalKeys
}

func getSampledKeyCount(ctx context.Context, client *redis.ClusterClient) int64 {
	// 클러스터에서 키 개수 가져오기 시도
	var totalKeys int64

	// 더 나은 샘플링 방식으로 CLUSTER COUNTKEYSINSLOT 사용
	// 16384 슬롯을 20개 구간으로 나누어 샘플링
	var sampleSlots []int
	for i := 0; i < 20; i++ {
		slot := (16384 * i) / 20
		sampleSlots = append(sampleSlots, slot)
	}

	var sampleCount int64
	validSamples := 0

	for _, slot := range sampleSlots {
		result := client.ClusterCountKeysInSlot(ctx, slot)
		if result.Err() == nil {
			sampleCount += result.Val()
			validSamples++
		}
	}

	if validSamples > 0 {
		// 샘플링 기반으로 전체 키 추정
		avgKeysPerSlot := float64(sampleCount) / float64(validSamples)
		totalKeys = int64(avgKeysPerSlot * 16384)
	}

	return totalKeys
}

func displayClusterReport(status *ClusterStatus, verbose bool) {
	fmt.Println(styles.TitleStyle.Render("클러스터 개요"))

	// 기본 통계
	fmt.Printf("총 노드: %s\n", styles.HighlightStyle.Render(strconv.Itoa(len(status.Nodes))))
	fmt.Printf("마스터: %s, 레플리카: %s\n",
		styles.SuccessStyle.Render(strconv.Itoa(status.Masters)),
		styles.InfoStyle.Render(strconv.Itoa(status.Replicas)))
	fmt.Printf("슬롯 커버리지: %s/%s (%s)\n",
		styles.HighlightStyle.Render(strconv.Itoa(status.TotalSlots)),
		styles.HighlightStyle.Render("16384"),
		styles.HighlightStyle.Render(fmt.Sprintf("%.1f%%", float64(status.TotalSlots)/163.84)))

	if status.TotalKeys > 0 {
		keyLabel := "예상 키 수"
		if status.PreciseKeyCount {
			keyLabel = "정확한 키 수"
		}
		fmt.Printf("%s: %s\n", keyLabel, styles.HighlightStyle.Render(formatNumber(status.TotalKeys)))
	}

	if status.FailedNodes > 0 {
		fmt.Printf("실패한 노드: %s\n", styles.ErrorStyle.Render(strconv.Itoa(status.FailedNodes)))
	}

	// 추가 클러스터 통계
	if status.ClusterState != "" {
		stateStyle := styles.SuccessStyle
		stateInfo := status.ClusterState

		if status.ClusterState == "fail" {
			stateStyle = styles.ErrorStyle
			stateInfo = fmt.Sprintf("%s (재확인 권장)", status.ClusterState)
		} else if status.ClusterState != "ok" {
			stateStyle = styles.WarningStyle
		}

		fmt.Printf("클러스터 상태: %s\n", stateStyle.Render(stateInfo))

		// 상태가 'fail'인 경우 추가 정보 제공
		if status.ClusterState == "fail" {
			fmt.Printf("  💡 %s\n",
				styles.DescStyle.Render("'fail' 상태는 종종 일시적입니다 (노드 간 동기화 지연)"))
			fmt.Printf("  💡 %s\n",
				styles.DescStyle.Render("몇 초 후 다시 확인하면 'ok'로 변경될 수 있습니다"))
		}
	}

	if status.KnownNodesCount > 0 && status.KnownNodesCount != len(status.Nodes) {
		fmt.Printf("알려진 노드 수: %s (실제: %s)\n",
			styles.WarningStyle.Render(strconv.Itoa(status.KnownNodesCount)),
			styles.HighlightStyle.Render(strconv.Itoa(len(status.Nodes))))
	}

	if status.ClusterSize > 0 {
		fmt.Printf("클러스터 크기: %s\n", styles.HighlightStyle.Render(strconv.Itoa(status.ClusterSize)))
	}

	if status.CurrentEpoch > 0 {
		fmt.Printf("현재 에포크: %s\n", styles.HighlightStyle.Render(strconv.FormatInt(status.CurrentEpoch, 10)))
	}

	fmt.Println()

	// 노드 상세
	fmt.Println(styles.TitleStyle.Render("노드 상세"))

	// 노드 정렬: 마스터 먼저, 그 다음 레플리카
	sortedNodes := append([]ClusterNode{}, status.Nodes...)
	sort.Slice(sortedNodes, func(i, j int) bool {
		if sortedNodes[i].IsMaster && !sortedNodes[j].IsMaster {
			return true
		}
		if !sortedNodes[i].IsMaster && sortedNodes[j].IsMaster {
			return false
		}
		return sortedNodes[i].Addr < sortedNodes[j].Addr
	})

	for _, node := range sortedNodes {
		displayNodeInfo(node, verbose)
	}
}

func displayNodeInfo(node ClusterNode, verbose bool) {
	var nodeType string
	var styledType string

	if node.IsFail {
		nodeType = "실패"
		styledType = styles.ErrorStyle.Render(nodeType)
	} else if node.IsMaster {
		nodeType = "마스터"
		styledType = styles.SuccessStyle.Render(nodeType)
	} else if node.IsReplica {
		nodeType = "레플리카"
		styledType = styles.InfoStyle.Render(nodeType)
	} else {
		nodeType = "알 수 없음"
		styledType = styles.WarningStyle.Render(nodeType)
	}

	// 주소 정리 (클러스터 포트 제거)
	addr := node.Addr
	if strings.Contains(addr, "@") {
		addr = strings.Split(addr, "@")[0]
	}

	// 주소 유효성 검사 및 정리
	if addr == "" || addr == ":0" || addr == ":" || strings.HasPrefix(addr, ":") {
		addr = "주소 불명"
	} else {
		// 유효한 주소인지 기본 검증
		parts := strings.Split(addr, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || parts[1] == "0" {
			addr = fmt.Sprintf("%s (주소 오류)", addr)
		}
	}

	if verbose {
		// Verbose 모드: 전체 상세 정보 표시
		fmt.Printf("  %s %s\n",
			styledType,
			styles.HighlightStyle.Render(addr))
		fmt.Printf("    ID: %s\n", styles.DescStyle.Render(node.ID))
		fmt.Printf("    주소: %s\n", styles.DescStyle.Render(node.Addr))

		if len(node.Flags) > 0 {
			fmt.Printf("    플래그: %s\n", styles.DescStyle.Render(strings.Join(node.Flags, ",")))
		}

		if len(node.Slots) > 0 {
			slotRanges := formatCheckSlotRanges(node.Slots)
			fmt.Printf("    슬롯: %s개", styles.HighlightStyle.Render(strconv.Itoa(len(node.Slots))))
			if len(slotRanges) <= 5 {
				fmt.Printf(" (%s)", strings.Join(slotRanges, ", "))
			} else {
				fmt.Printf(" (%s, ...)", strings.Join(slotRanges[:5], ", "))
			}
			fmt.Println()
		}

		if node.IsReplica && node.MasterID != "" {
			fmt.Printf("    마스터 ID: %s\n", styles.DescStyle.Render(node.MasterID))
		}
		fmt.Println()
	} else {
		// 컴팩트 모드: 원래 포맷
		fmt.Printf("  %s %s | %s",
			styledType,
			styles.HighlightStyle.Render(addr),
			styles.DescStyle.Render(node.ID[:8]+"..."))

		if len(node.Slots) > 0 {
			slotRanges := formatCheckSlotRanges(node.Slots)
			fmt.Printf(" | 슬롯: %s", styles.HighlightStyle.Render(strconv.Itoa(len(node.Slots))))
			if len(slotRanges) <= 3 {
				fmt.Printf(" (%s)", strings.Join(slotRanges, ", "))
			}
		}

		if node.IsReplica && node.MasterID != "" {
			fmt.Printf(" | 마스터: %s", styles.DescStyle.Render(node.MasterID[:8]+"..."))
		}

		fmt.Println()
	}
}

func formatCheckSlotRanges(slots []int) []string {
	if len(slots) == 0 {
		return []string{}
	}

	sort.Ints(slots)
	var ranges []string
	start := slots[0]
	end := slots[0]

	for i := 1; i < len(slots); i++ {
		if slots[i] == end+1 {
			end = slots[i]
		} else {
			if start == end {
				ranges = append(ranges, strconv.Itoa(start))
			} else {
				ranges = append(ranges, fmt.Sprintf("%d-%d", start, end))
			}
			start = slots[i]
			end = slots[i]
		}
	}

	// 마지막 범위 추가
	if start == end {
		ranges = append(ranges, strconv.Itoa(start))
	} else {
		ranges = append(ranges, fmt.Sprintf("%d-%d", start, end))
	}

	return ranges
}

func runHealthChecks(status *ClusterStatus) []string {
	fmt.Print(styles.InfoStyle.Render("3. 기본 건강성 검사 실행..."))

	var issues []string

	// 체크 1: 모든 슬롯 커버되었는지
	if status.TotalSlots != 16384 {
		issues = append(issues, fmt.Sprintf("슬롯 커버리지 불완전: %d/16384 슬롯", status.TotalSlots))
	}

	// 체크 2: 실패한 노드들
	if status.FailedNodes > 0 {
		issues = append(issues, fmt.Sprintf("실패한 노드: %d개", status.FailedNodes))
	}

	// 체크 3: 클러스터 상태 (fail이지만 슬롯이 완전히 커버되어 있으면 경고 수준으로)
	if status.ClusterState == "fail" {
		if status.TotalSlots == 16384 && status.FailedNodes == 0 {
			// 일시적 fail 상태로 추정
			issues = append(issues, "클러스터 상태 'fail' (일시적 상태 변화 가능성 - 재확인 권장)")
		} else {
			// 실제 문제가 있는 fail 상태
			issues = append(issues, fmt.Sprintf("클러스터 상태 '%s' (심각: 슬롯 또는 노드 문제)", status.ClusterState))
		}
	}

	// 체크 4: 복제본 없는 마스터들
	mastersWithoutReplicas := 0
	for _, node := range status.Nodes {
		if node.IsMaster && !node.IsFail {
			hasReplica := false
			for _, otherNode := range status.Nodes {
				if otherNode.IsReplica && otherNode.MasterID == node.ID && !otherNode.IsFail {
					hasReplica = true
					break
				}
			}
			if !hasReplica {
				mastersWithoutReplicas++
			}
		}
	}

	if mastersWithoutReplicas > 0 {
		issues = append(issues, fmt.Sprintf("복제본 없는 마스터: %d개 (고가용성 위험)", mastersWithoutReplicas))
	}

	// 체크 5: 슬롯 분배 불균형
	if status.Masters > 1 {
		slotCounts := make([]int, 0, status.Masters)
		for _, node := range status.Nodes {
			if node.IsMaster && !node.IsFail {
				slotCounts = append(slotCounts, len(node.Slots))
			}
		}

		if len(slotCounts) > 0 {
			minSlots := slotCounts[0]
			maxSlots := slotCounts[0]
			for _, count := range slotCounts {
				if count < minSlots {
					minSlots = count
				}
				if count > maxSlots {
					maxSlots = count
				}
			}

			// 더 합리적인 임계값: 다음 경우에만 불균형으로 표시:
			// 1. 평균의 20% 이상 차이 AND
			// 2. 1000 슬롯 이상 차이 (절대값으로 의미 있는 차이)
			avgSlots := 16384 / status.Masters
			threshold := avgSlots / 5 // 평균의 20%
			if threshold < 1000 {
				threshold = 1000 // 최소 1000 슬롯 임계값
			}

			if maxSlots-minSlots > threshold {
				issues = append(issues, fmt.Sprintf("슬롯 분배 불균형: 최소 %d, 최대 %d 슬롯", minSlots, maxSlots))
			}
		}
	}

	// 체크 6: 핸드셰이크 상태 노드들
	handshakeNodes := 0
	for _, node := range status.Nodes {
		if node.IsHandshake {
			handshakeNodes++
		}
	}

	if handshakeNodes > 0 {
		issues = append(issues, fmt.Sprintf("핸드셰이크 상태 노드: %d개 (연결 중)", handshakeNodes))
	}

	// 체크 7: 주소 정보가 잘못된 노드들
	malformedAddressNodes := 0
	for _, node := range status.Nodes {
		addr := node.Addr
		if strings.Contains(addr, "@") {
			addr = strings.Split(addr, "@")[0]
		}

		if addr == "" || addr == ":0" || addr == ":" || strings.HasPrefix(addr, ":") {
			malformedAddressNodes++
		} else {
			parts := strings.Split(addr, ":")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" || parts[1] == "0" {
				malformedAddressNodes++
			}
		}
	}

	if malformedAddressNodes > 0 {
		issues = append(issues, fmt.Sprintf("주소 정보 오류 노드: %d개 (클러스터 동기화 문제 가능성)", malformedAddressNodes))
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return issues
}

func displayHealthReport(issues []string) {
	fmt.Println()
	fmt.Println(styles.TitleStyle.Render("건강성 보고서"))

	if len(issues) == 0 {
		fmt.Println(styles.SuccessStyle.Render("모든 건강성 검사 통과"))
		fmt.Println()
		fmt.Println(styles.SuccessStyle.Render("클러스터가 정상 상태입니다!"))
	} else {
		fmt.Println(styles.WarningStyle.Render("  발견된 문제들:"))
		for i, issue := range issues {
			fmt.Printf("  %d. %s\n", i+1, styles.WarningStyle.Render(issue))
		}
		fmt.Println()
		fmt.Printf(styles.WarningStyle.Render("⚠️  발견된 문제: %d개\n"), len(issues))
	}
	fmt.Println() // Add final newline
}

func checkClusterConsistency(ctx context.Context, client *redis.ClusterClient, status *ClusterStatus) ([]string, error) {
	fmt.Print(styles.InfoStyle.Render("4. 클러스터 정보 일관성 검사..."))

	var issues []string
	user, password := config.GetAuth()

	// 각 노드에서 클러스터 노드 출력 저장
	nodeClusterInfo := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 병렬로 각 연결 가능한 노드에서 클러스터 노드 정보 가져오기
	for _, node := range status.Nodes {
		if node.IsFail {
			continue // 실패한 노드는 건너뛰기
		}

		wg.Add(1)
		go func(nodeAddr string) {
			defer wg.Done()

			// 주소 정리
			addr := nodeAddr
			if strings.Contains(addr, "@") {
				addr = strings.Split(addr, "@")[0]
			}

			// 개별 노드에 연결 (타임아웃 설정)
			nodeClient := redis.NewClient(&redis.Options{
				Addr:         addr,
				Username:     user,
				Password:     password,
				DialTimeout:  time.Second * 3,
				ReadTimeout:  time.Second * 3,
				WriteTimeout: time.Second * 3,
			})
			defer nodeClient.Close()

			// 이 특정 노드에서 CLUSTER NODES 가져오기
			result := nodeClient.ClusterNodes(ctx)
			if result.Err() != nil {
				mu.Lock()
				issues = append(issues, fmt.Sprintf("노드 %s에서 클러스터 정보 조회 실패: %v", addr, result.Err()))
				mu.Unlock()
				return
			}

			mu.Lock()
			nodeClusterInfo[addr] = normalizeClusterNodesOutput(result.Val())
			mu.Unlock()
		}(node.Addr)
	}

	wg.Wait()

	// 모든 클러스터 노드 출력의 일관성 비교
	if len(nodeClusterInfo) > 1 {
		var referenceOutput string
		var referenceNode string

		// 첫 번째 노드의 출력을 기준으로 삼기
		for addr, output := range nodeClusterInfo {
			referenceOutput = output
			referenceNode = addr
			break
		}

		// 다른 모든 노드들을 기준과 비교
		inconsistentNodes := []string{}
		for addr, output := range nodeClusterInfo {
			if addr != referenceNode && output != referenceOutput {
				inconsistentNodes = append(inconsistentNodes, addr)
			}
		}

		if len(inconsistentNodes) > 0 {
			// 클러스터 상태가 OK이고 모든 슬롯이 커버되어 있다면 정보성으로 처리
			if status.ClusterState == "ok" && status.TotalSlots == 16384 && status.FailedNodes == 0 {
				// 클러스터가 건강하면 이걸 문제로 보고하지 않음 - 단순한 타이밍 차이일 가능성 높음
				fmt.Printf(" %s (일관성 차이 감지됨, 클러스터 정상 작동)\n", styles.SuccessStyle.Render("완료"))
			} else {
				issues = append(issues, fmt.Sprintf("클러스터 정보 불일치: %d개 노드가 다른 클러스터 뷰를 가지고 있음 (%s)",
					len(inconsistentNodes), strings.Join(inconsistentNodes, ", ")))
			}
		}
	}

	if len(issues) == 0 {
		fmt.Println(styles.SuccessStyle.Render(" 완료"))
	}
	return issues, nil
}

func normalizeClusterNodesOutput(output string) string {
	lines := strings.Split(output, "\n")
	var normalizedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 일관성 있는 비교를 위해 노드 라인 파싱하고 정렬
		// 일관성 체크를 위해 모든 동적/변수 부분 제거
		parts := strings.Fields(line)
		if len(parts) >= 8 {
			// 포맷: <id> <ip:port@cport> <flags> <master> <ping-sent> <pong-recv> <config-epoch> <link-state> <slot1> ... <slotN>
			// 진짜 안정적인 부분들만 유지: ID(0), addr(1), normalized_flags(2), master(3), slots(8+)

			var stableParts []string
			stableParts = append(stableParts, parts[0]) // ID

			// 주소 정규화 (클러스터 포트 제거)
			addr := parts[1]
			if strings.Contains(addr, "@") {
				addr = strings.Split(addr, "@")[0]
			}
			stableParts = append(stableParts, addr)

			// 플래그 정규화 (노드별 특성인 myself, handshake 플래그 제거)
			flags := strings.Split(parts[2], ",")
			var normalizedFlags []string
			for _, flag := range flags {
				if flag != "myself" && flag != "handshake" {
					normalizedFlags = append(normalizedFlags, flag)
				}
			}
			sort.Strings(normalizedFlags) // 일관성을 위해 플래그 정렬
			stableParts = append(stableParts, strings.Join(normalizedFlags, ","))

			stableParts = append(stableParts, parts[3]) // master

			// 슬롯 추가 (있다면) - 이것들은 일관되어야 함
			if len(parts) > 8 {
				stableParts = append(stableParts, parts[8:]...)
			}

			normalizedLines = append(normalizedLines, strings.Join(stableParts, " "))
		}
	}

	// 일관된 비교를 위해 라인 정렬
	sort.Strings(normalizedLines)
	return strings.Join(normalizedLines, "\n")
}

// getClusterStatusQuick quickly checks only the cluster state without full analysis
func getClusterStatusQuick(ctx context.Context, client *redis.ClusterClient) (string, error) {
	infoResult := client.ClusterInfo(ctx)
	if infoResult.Err() != nil {
		return "", infoResult.Err()
	}

	infoLines := strings.Split(infoResult.Val(), "\n")
	for _, line := range infoLines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "cluster_state:") {
			if parts := strings.Split(line, ":"); len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "unknown", nil
}
