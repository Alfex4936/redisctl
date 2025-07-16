package cmd

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"

	"redisctl/internal/config"
	"redisctl/internal/styles"
)

// NewPopulateCommand 'populate-test-data' 명령어
func NewPopulateCommand() *cobra.Command {
	var numKeys int

	cmd := &cobra.Command{
		Use:   "populate-test-data [--num-keys N] <cluster-node-ip:port>",
		Short: "p 클러스터에 테스트 데이터를 삽입합니다",
		Long: styles.TitleStyle.Render("[TEST] Redis 클러스터 테스트 데이터 생성") + "\n\n" +
			styles.DescStyle.Render("Redis 클러스터에 성능 테스트용 더미 데이터를 생성합니다.") + "\n\n" +
			styles.DescStyle.Render("생성되는 데이터:") + "\n" +
			styles.DescStyle.Render("• 키: key:0000000001, key:0000000002, ...") + "\n" +
			styles.DescStyle.Render("• 값: val:0000000001, val:0000000002, ...") + "\n" +
			styles.DescStyle.Render("• 클러스터 전체에 자동 분산") + "\n" +
			styles.DescStyle.Render("• 병렬 처리로 빠른 삽입"),
		Example: `  # 기본 1,000개 키 생성
  redisctl populate-test-data localhost:7001

  # 100,000개 키 생성
  redisctl --password mypass populate-test-data --num-keys 100000 localhost:9001

  # 최대 10,000,000개 키 생성 (대규모 테스트)
  redisctl --password mypass populate-test-data --num-keys 10000000 localhost:9001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.ValidateAuth(); err != nil {
				return err
			}
			return runPopulateTestData(args[0], numKeys)
		},
	}

	cmd.Flags().IntVar(&numKeys, "num-keys", 1000, "삽입할 키 수 (기본값: 1,000, 최대: 10,000,000)")

	return cmd
}

type PopulateStats struct {
	TotalKeys     int
	ProcessedKeys int
	ErrorKeys     int
	StartTime     time.Time
	ElapsedTime   time.Duration
	KeysPerSecond float64
	WorkerCount   int
}

func runPopulateTestData(clusterAddr string, numKeys int) error {
	// Validate input
	if numKeys <= 0 {
		return fmt.Errorf("키 수는 1 이상이어야 합니다")
	}
	if numKeys > 10000000 {
		return fmt.Errorf("최대 키 수는 10,000,000개입니다")
	}

	fmt.Println(styles.InfoStyle.Render("Redis 클러스터 테스트 데이터 생성"))
	fmt.Printf("클러스터: %s\n", styles.HighlightStyle.Render(clusterAddr))
	fmt.Printf("생성할 키 수: %s\n", styles.HighlightStyle.Render(formatNumber(int64(numKeys))))
	fmt.Println()

	// Connect to cluster
	user, password := config.GetAuth()
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    []string{clusterAddr},
		Username: user,
		Password: password,
	})
	defer client.Close()

	ctx := context.Background()

	// Validate cluster connectivity
	if err := validatePopulateConnectivity(ctx, client); err != nil {
		return fmt.Errorf("클러스터 연결 실패: %w", err)
	}

	// Check cluster status
	if err := validateClusterForPopulate(ctx, client); err != nil {
		return fmt.Errorf("클러스터 상태 확인 실패: %w", err)
	}

	// Estimate optimal worker count based on cluster size and data size
	workerCount := calculateOptimalWorkerCount(numKeys)
	fmt.Printf("병렬 작업자 수: %s\n", styles.HighlightStyle.Render(strconv.Itoa(workerCount)))
	fmt.Println()

	// Initialize statistics
	stats := &PopulateStats{
		TotalKeys:   numKeys,
		StartTime:   time.Now(),
		WorkerCount: workerCount,
	}

	// Run data population with progress tracking
	if err := populateDataWithProgress(ctx, client, stats); err != nil {
		return fmt.Errorf("데이터 생성 실패: %w", err)
	}

	// Display final statistics
	displayPopulateResults(stats)

	fmt.Println()
	fmt.Println(styles.SuccessStyle.Render("테스트 데이터 생성이 완료되었습니다!"))
	return nil
}

func validatePopulateConnectivity(ctx context.Context, client *redis.ClusterClient) error {
	fmt.Print(styles.InfoStyle.Render("1. 클러스터 연결 확인..."))

	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return err
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return nil
}

func validateClusterForPopulate(ctx context.Context, client *redis.ClusterClient) error {
	fmt.Print(styles.InfoStyle.Render("2. 클러스터 상태 확인..."))

	// Check cluster info
	info := client.ClusterInfo(ctx)
	if info.Err() != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return info.Err()
	}

	// Check if cluster is in OK state
	infoStr := info.Val()
	if !contains(infoStr, "cluster_state:ok") {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return fmt.Errorf("클러스터가 정상 상태가 아닙니다")
	}

	// Check cluster nodes
	nodes := client.ClusterNodes(ctx)
	if nodes.Err() != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return nodes.Err()
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return nil
}

func calculateOptimalWorkerCount(numKeys int) int {
	// Base worker count on data size and optimal resource usage
	if numKeys <= 1000 {
		return 2 // Small datasets - minimal overhead
	} else if numKeys <= 10000 {
		return 4 // Small to medium
	} else if numKeys <= 50000 {
		return 8 // Medium datasets
	} else if numKeys <= 100000 {
		return 12 // Large datasets
	} else if numKeys <= 1000000 {
		return 20 // Very large datasets
	} else {
		return 30 // Massive datasets - balance between speed and resource usage
	}
}

func populateDataWithProgress(ctx context.Context, client *redis.ClusterClient, stats *PopulateStats) error {
	fmt.Println(styles.InfoStyle.Render("3. 테스트 데이터 생성 중..."))

	// Calculate optimal batch size based on total keys
	batchSize := calculateOptimalBatchSize(stats.TotalKeys)

	// Create work channel
	workChan := make(chan int, stats.WorkerCount*2)
	resultChan := make(chan PopulateResult, stats.WorkerCount)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < stats.WorkerCount; i++ {
		wg.Add(1)
		go populateWorkerWithBatchSize(ctx, client, workChan, resultChan, &wg, batchSize)
	}

	// Send work items
	go func() {
		defer close(workChan)
		for i := 1; i <= stats.TotalKeys; i++ {
			workChan <- i
		}
	}()

	// Progress tracking
	progressTicker := time.NewTicker(time.Second)
	defer progressTicker.Stop()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var completed int
	var errors int

	for {
		select {
		case result, ok := <-resultChan:
			if !ok {
				// All workers finished
				stats.ProcessedKeys = completed
				stats.ErrorKeys = errors
				stats.ElapsedTime = time.Since(stats.StartTime)
				if stats.ElapsedTime.Seconds() > 0 {
					stats.KeysPerSecond = float64(completed) / stats.ElapsedTime.Seconds()
				}
				return nil
			}

			if result.Success {
				completed++
			} else {
				errors++
			}

		case <-progressTicker.C:
			// Display progress
			total := completed + errors
			if total > 0 {
				percentage := float64(total) / float64(stats.TotalKeys) * 100
				elapsed := time.Since(stats.StartTime)
				rate := float64(total) / elapsed.Seconds()

				fmt.Printf("\r진행률: %s/%s (%.1f%%) | 속도: %.0f keys/sec | 오류: %d",
					styles.HighlightStyle.Render(formatNumber(int64(total))),
					styles.HighlightStyle.Render(formatNumber(int64(stats.TotalKeys))),
					percentage,
					rate,
					errors)
			}
		}
	}
}

func calculateOptimalBatchSize(totalKeys int) int {
	if totalKeys <= 1000 {
		return 50
	} else if totalKeys <= 10000 {
		return 100
	} else if totalKeys <= 100000 {
		return 200
	} else {
		return 500
	}
}

func populateWorkerWithBatchSize(ctx context.Context, client *redis.ClusterClient, workChan <-chan int, resultChan chan<- PopulateResult, wg *sync.WaitGroup, batchSize int) {
	defer wg.Done()

	// Pre-allocate slices to avoid repeated allocations
	batch := make([]int, 0, batchSize)
	keys := make([]string, 0, batchSize)
	values := make([]string, 0, batchSize)

	for keyIndex := range workChan {
		// Collect items for batch
		batch = append(batch, keyIndex)
		keys = append(keys, fmt.Sprintf("key:%010d", keyIndex))
		values = append(values, fmt.Sprintf("val:%010d", keyIndex))

		// Execute batch when full or channel closed
		if len(batch) >= batchSize {
			executeBatch(ctx, client, batch, keys, values, resultChan)
			// Reset batch (keep capacity)
			batch = batch[:0]
			keys = keys[:0]
			values = values[:0]
		}
	}

	// Execute remaining items in final batch
	if len(batch) > 0 {
		executeBatch(ctx, client, batch, keys, values, resultChan)
	}
}

type PopulateResult struct {
	KeyIndex int
	Success  bool
	Error    error
}

func executeBatch(ctx context.Context, client *redis.ClusterClient, batch []int, keys []string, values []string, resultChan chan<- PopulateResult) {
	// Create pipeline for batch execution
	pipeline := client.Pipeline()

	// Add all SET commands to pipeline
	for i := 0; i < len(batch); i++ {
		pipeline.Set(ctx, keys[i], values[i], 0)
	}

	// Execute entire batch in single network round trip
	results, err := pipeline.Exec(ctx)

	if err != nil {
		// If pipeline fails, mark all items as failed
		for _, keyIndex := range batch {
			resultChan <- PopulateResult{
				KeyIndex: keyIndex,
				Success:  false,
				Error:    err,
			}
		}
		return
	}

	// Process individual results from pipeline
	for i, result := range results {
		success := result.Err() == nil
		resultChan <- PopulateResult{
			KeyIndex: batch[i],
			Success:  success,
			Error:    result.Err(),
		}
	}
}

func displayPopulateResults(stats *PopulateStats) {
	fmt.Printf("\n\n")
	fmt.Println(styles.TitleStyle.Render("생성 결과"))

	successRate := float64(stats.ProcessedKeys) / float64(stats.TotalKeys) * 100

	fmt.Printf("총 키 수: %s\n", styles.HighlightStyle.Render(formatNumber(int64(stats.TotalKeys))))
	fmt.Printf("성공: %s (%.1f%%)\n",
		styles.SuccessStyle.Render(formatNumber(int64(stats.ProcessedKeys))),
		successRate)

	if stats.ErrorKeys > 0 {
		errorRate := float64(stats.ErrorKeys) / float64(stats.TotalKeys) * 100
		fmt.Printf("실패: %s (%.1f%%)\n",
			styles.ErrorStyle.Render(formatNumber(int64(stats.ErrorKeys))),
			errorRate)

		if errorRate > 5.0 {
			fmt.Printf("  높은 실패율 감지: 클러스터 상태나 네트워크를 확인하세요\n")
		}
	}

	fmt.Printf("소요 시간: %s\n", styles.HighlightStyle.Render(formatDuration(stats.ElapsedTime)))
	fmt.Printf("처리 속도: %s keys/sec\n", styles.HighlightStyle.Render(formatNumber(int64(stats.KeysPerSecond))))
	fmt.Printf("작업자 수: %s\n", styles.InfoStyle.Render(strconv.Itoa(stats.WorkerCount)))

	// Performance analysis with more detailed feedback
	if stats.KeysPerSecond > 10000 {
		fmt.Printf("성능: %s (클러스터 최적화 상태)\n", styles.SuccessStyle.Render("우수"))
	} else if stats.KeysPerSecond > 5000 {
		fmt.Printf("성능: %s (정상 동작 범위)\n", styles.InfoStyle.Render("양호"))
	} else if stats.KeysPerSecond > 1000 {
		fmt.Printf("성능: %s (병목 가능성 있음)\n", styles.WarningStyle.Render("보통"))
	} else {
		fmt.Printf("성능: %s (클러스터 상태 점검 필요)\n", styles.ErrorStyle.Render("느림"))
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1000000)
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
