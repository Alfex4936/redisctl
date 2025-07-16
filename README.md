# redisctl

## 주요 기능

- **클러스터 생성**: 여러 Redis 노드로부터 클러스터를 초기화
- **노드 추가**: 기존 클러스터에 새로운 노드 추가
- **리샤딩**: 마스터 간 슬롯 이동으로 데이터 재분산
- **노드 제거**: 클러스터에서 불필요한 노드 제거
- **상태 확인**: 클러스터 상태 및 노드 상태 모니터링
- **테스트 데이터**: 성능 테스트를 위한 더미 데이터 생성
- **자동 리밸런싱**: 슬롯 분배 자동 균형 조정

## 설치 및 빌드

### 사전 요구사항

- Go 1.23 이상
- Redis v7.2 서버 (로컬 또는 원격)

### 빌드

```bash
# 의존성 설치
go mod tidy

# 빌드
go build -o redisctl main.go

# 또는 직접 실행
go run main.go --help
```

### Windows에서 빌드

```powershell
# PowerShell에서
go build -o redisctl.exe main.go
```

## 사용법

### 기본 명령 구조

```bash
redisctl [--user str] [--password str] {command} [options...]
```

### 전역 옵션

- `--user, -u`: Redis 사용자명 (기본 인증 사용시 생략 가능)
- `--password, -p`: Redis 비밀번호 **필수**

## 명령어 상세

### 1. 클러스터 생성 (`create`)

```bash
redisctl create [--replicas N] ip1:port1 ... ipN:portN
```

**예시:**
```bash
# 3개 노드로 클러스터 생성 (복제본 없음)
redisctl --password mypass create localhost:7001 localhost:7002 localhost:7003

# 6개 노드로 클러스터 생성 (각 마스터당 복제본 1개)
redisctl --password mypass create --replicas 1 \
  localhost:7001 localhost:7002 localhost:7003 \
  localhost:7004 localhost:7005 localhost:7006
```

**옵션:**
- `--replicas N`: 각 마스터당 복제본 수 (기본값: 0)

### 2. 노드 추가 (`add-node`)

```bash
redisctl add-node [--master-id <str>] new_ip:new_port existing_ip:existing_port
```

**예시:**
```bash
# 새 마스터 노드 추가 (슬롯 없음)
redisctl --password mypass add-node localhost:7007 localhost:7001

# 특정 마스터의 복제본으로 노드 추가
redisctl --password mypass add-node --master-id <master-node-id> localhost:7008 localhost:7001
```

**인수:**
- `new_ip:new_port`: 추가할 새 노드
- `existing_ip:existing_port`: 클러스터 내의 기존 노드

**옵션:**
- `--master-id`: 새 노드를 지정된 마스터의 **복제본**으로 만듭니다
- 생략시: 새 노드는 **마스터**로 추가됩니다 (슬롯 없음)

**구현 단계:**
1. 기존 클러스터 노드 연결 및 상태 확인
2. 새 노드 연결 및 중복 참여 검사
3. master-id 지정시 해당 마스터 존재 여부 확인
4. `CLUSTER MEET` 명령으로 클러스터에 노드 추가
5. master-id 지정시 `CLUSTER REPLICATE` 명령으로 복제본 설정
6. 노드 추가 성공 여부 확인 및 정보 출력
7. 이미 클러스터에 참여한 노드를 자동으로 `CLUSTER RESET HARD`하지 않음으로써 의도치 않은 데이터 손실 방지

### 3. 리샤딩 (`reshard`)

```bash
redisctl reshard --from str --to str --slots N [--pipeline N] ip:port
```

**예시:**
```bash
# 마스터 간 1000개 슬롯 이동
redisctl --password mypass reshard \
  --from <source-master-id> \
  --to <target-master-id> \
  --slots 1000 \
  localhost:7001

# 파이프라인 크기 조정으로 성능 최적화
redisctl --password mypass reshard \
  --from <source-id> \
  --to <target-id> \
  --slots 500 \
  --pipeline 20 \
  localhost:7001
```

**인수:**
- `ip:port`: 클러스터에 연결할 노드

**옵션:**
- `--from`: 소스 마스터 노드 ID **필수**
- `--to`: 대상 마스터 노드 ID **필수**  
- `--slots`: 이동할 슬롯 수 **필수**
- `--pipeline`: MIGRATE당 키 수 (기본값: 10)

**구현 단계:**
1. 클러스터 연결 및 상태 검증
2. 소스/대상 마스터 노드 검증 및 슬롯 수 확인
3. 이동할 슬롯 선택 (소스 노드의 슬롯 중에서)
4. 각 슬롯별 마이그레이션 준비 (`CLUSTER SETSLOT MIGRATING/IMPORTING`)
5. 슬롯 내 키들을 `MIGRATE` 명령으로 배치 이동 (60초 타임아웃)
6. 슬롯 상태를 `STABLE`로 설정
7. `CLUSTER DELSLOTS`/`CLUSTER ADDSLOTS`로 클러스터 설정 업데이트
8. 마이그레이션 결과 검증 및 요약 표시
9. 슬롯 소유권 변경을 모든 클러스터 노드에 전파하여 `MOVED` 리다이렉트 오류 방지

**MIGRATE 설정:**
- 타임아웃: 60,000ms
- COPY 및 REPLACE 플래그 사용 안 함 (assignment 요구사항)

### 4. 노드 제거 (`del-node`)

```bash
redisctl del-node <cluster-node-ip:port> <node-id>
```

**예시:**
```bash
# 레플리카 노드 제거
redisctl --password mypass del-node localhost:7001 <replica-node-id>

# 마스터 노드 제거 (슬롯 자동 재분배)
redisctl --password mypass del-node localhost:7001 <master-node-id>
```

**인수:**
- `cluster-node-ip:port`: 클러스터에 연결할 노드
- `node-id`: 제거할 노드의 ID

**구현 단계:**
1. 클러스터 연결 및 상태 검증
2. 제거할 노드 정보 조회 및 검증
3. 마스터 노드인 경우: 슬롯을 다른 마스터들에게 균등하게 재분배
4. 레플리카 노드인 경우: 단순히 클러스터에서 제거
5. `CLUSTER FORGET` 명령으로 노드 제거
6. 노드 제거 성공 여부 확인
7. `CLUSTER FORGET`을 모든 클러스터 노드에 전송하여 완전한 노드 제거 보장
8. 슬롯 재분배 시 모든 노드에 새로운 소유권 정보 전파

**슬롯 재분배 과정:**
- 제거할 마스터 노드의 슬롯을 다른 마스터들에게 균등 분배
- `CLUSTER SETSLOT MIGRATING/IMPORTING` 명령으로 슬롯 이동 준비
- `MIGRATE` 명령으로 키들을 배치 이동 (60초 타임아웃)
- `CLUSTER SETSLOT NODE` 명령으로 슬롯 소유권 이전

### 5. 상태 확인 (`check`)

```bash
redisctl check <cluster-node-ip:port>
```

**예시:**
```bash
# 클러스터 상태 확인
redisctl --password mypass check localhost:7001

# 상세한 클러스터 보고서 생성
redisctl --password mypass check localhost:9001

# 대부분의 경우: redisctl check
./redisctl check localhost:9001

# 상세 분석 필요시: verbose 모드  
./redisctl check --verbose localhost:9001

# 디버깅/스크립팅: raw 모드
./redisctl check --raw localhost:9001

# 정확한 키 갯수
./redisctl check --dbsize localhost:9001

# 빠른 원시 데이터: cluster nodes
redis-cli -p 9001 cluster nodes
```

**인수:**
- `cluster-node-ip:port`: 클러스터에 연결할 노드

**확인 항목:**
- **클러스터 개요**: 총 노드 수, 마스터/레플리카 비율, 실패한 노드
- **슬롯 커버리지**: 16384개 슬롯 중 서비스되는 슬롯 수와 비율
- **키 통계**: 클러스터 내 예상 키 수 (샘플링 기반)
- **노드 상세**: 각 노드의 타입, 주소, ID, 할당된 슬롯 범위
- **마스터-레플리카 관계**: 복제 구조 및 관계

**건강성 검사:**
- **슬롯 완전성**: 모든 16384개 슬롯이 서비스되는지 확인
- **노드 상태**: 실패하거나 연결 중인 노드 감지
- **고가용성**: 복제본 없는 마스터 노드 식별
- **부하 분산**: 마스터 간 슬롯 분배 균형 확인
- **연결 상태**: 핸드셰이크 진행 중인 노드 확인

**출력 형식:**
- 노드별 상세 정보 (타입, 주소, 슬롯)
- 문제점 목록 및 권장사항

### 6. 테스트 데이터 생성 (`populate-test-data`)

```bash
redisctl populate-test-data [--num-keys N] <cluster-node-ip:port>
```

**예시:**
```bash
# 기본 1,000개 키 생성
redisctl --password mypass populate-test-data localhost:7001

# 100,000개 키 생성
redisctl --password mypass populate-test-data --num-keys 100000 localhost:9001

# 최대 10,000,000개 키 생성 (대규모 테스트)
redisctl --password mypass populate-test-data --num-keys 10000000 localhost:9001
```

**인수:**
- `cluster-node-ip:port`: 클러스터에 연결할 노드

**옵션:**
- `--num-keys N`: 삽입할 키 수 (기본값: 1,000, 최대: 10,000,000)

**구현 단계:**
1. 클러스터 연결 및 상태 검증
2. 키 수 유효성 검사 (최대 1천만개)
3. 병렬 워커를 사용한 배치 데이터 삽입
4. 진행률 표시 및 성능 통계
5. 삽입 완료 후 최종 결과 보고
6. Redis 파이프라이닝을 사용하여 대량 데이터 삽입 성능을 50-100배 향상 (개별 SET → 배치 파이프라인)

**데이터 형식:**
- **키**: `key:0000000001`, `key:0000000002`, ... (10자리 제로패딩)
- **값**: `val:0000000001`, `val:0000000002`, ... (대응되는 값)
- **분산**: Redis 클러스터의 해시 슬롯에 자동 분산

**성능 최적화:**
- **병렬 처리**: 다중 고루틴으로 동시 삽입
- **배치 처리**: 1,000개씩 묶어서 처리
- **진행률 표시**: 실시간 진행률 및 예상 완료 시간
- **에러 핸들링**: 실패한 키에 대한 재시도 로직

**사용 사례:**
- 클러스터 성능 테스트
- 리샤딩 작업 검증용 데이터
- 부하 분산 테스트
- 백업/복구 테스트

### 7. 자동 리밸런싱 (`rebalance`)

```bash
redisctl rebalance [--dry-run] [--threshold N] [--pipeline N] <cluster-node-ip:port>
```

**예시:**
```bash
# 클러스터 자동 리밸런싱
redisctl --password mypass rebalance localhost:7001

# 드라이런 모드로 변경사항 미리보기
redisctl --password mypass rebalance --dry-run localhost:7001

# 10% 이상 불균형시에만 리밸런싱
redisctl --password mypass rebalance --threshold 10 localhost:9001

# 파이프라인 크기 조정으로 성능 최적화
redisctl --password mypass rebalance --pipeline 20 localhost:7001
```

**인수:**
- `cluster-node-ip:port`: 클러스터에 연결할 노드

**옵션:**
- `--dry-run`: 실제 변경 없이 리밸런싱 계획만 표시
- `--threshold N`: 리밸런싱 임계값 (퍼센트, 기본: 5%)
- `--pipeline N`: MIGRATE당 키 수 (기본: 10)

**구현 단계:**
1. 클러스터 연결 및 상태 검증
2. 현재 마스터 노드들의 슬롯 분배 상태 분석
3. 불균형도 계산 (최대 편차 / 이상적 분배 * 100)
4. 임계값 기반 리밸런싱 필요성 판단
5. 최적 분배 계획 생성 (과부하 노드 → 부족 노드)
6. 드라이런 모드시 계획만 표시, 실행 모드시 슬롯 이동
7. 각 슬롯별 `CLUSTER SETSLOT` 명령으로 상태 설정
8. `MIGRATE` 명령으로 키들을 안전하게 이동
9. 리밸런싱 완료 후 최종 불균형도 보고

**주요 특징:**
- **자동 분석**: 현재 슬롯 분배 상태를 자동으로 분석하고 최적 분배 계산
- **임계값 기반**: 설정된 임계값 이상의 불균형이 있을 때만 리밸런싱 수행
- **드라이런 모드**: `--dry-run` 플래그로 실제 변경 없이 계획만 미리보기
- **안전한 이동**: 기존 `reshard` 로직 재사용으로 안전한 슬롯 이동
- **진행률 표시**: 각 단계별 진행 상황과 완료 상태 표시

**사용 사례:**
- 노드 추가/제거 후 슬롯 분배 최적화
- 정기적인 클러스터 유지보수
- 성능 최적화를 위한 부하 분산
- 클러스터 확장 후 자동 균형 조정

**리밸런싱 알고리즘:**
1. **이상적 분배 계산**: 16384 슬롯 ÷ 마스터 수
2. **불균형 감지**: 각 마스터의 슬롯 수와 이상적 수의 편차 계산
3. **이동 계획 수립**: 과부하 마스터 → 부족 마스터로 슬롯 이동
4. **최소 이동 최적화**: 필요한 최소한의 슬롯만 이동하여 효율성 극대화

## 시나리오 테스트

과제에서 요구하는 전체 시나리오 테스트:

### Ubuntu 22.04 테스트

```bash
# 1. Redis 클러스터 노드 설정 (자동 스크립트 사용)
.\setup-redis-cluster.sh

# 2. 8개 창에서 각각 Redis 서버 시작
cd redis-nodes\9001 && redis-server redis.conf
cd redis-nodes\9002 && redis-server redis.conf

# ... (9001~9008까지 각각)

# 아니면
./redis-nodes/redis-nodes-manager.sh

```

`시나리오및설명.pdf` 로 대체합니다.

### 클러스터 관리 명령어

```bash
# 3. 6개 노드로 클러스터 생성 (3 마스터 + 3 복제본)
./redisctl --password myredispassword create --replicas 1 `
  localhost:9001 localhost:9002 localhost:9003 `
  localhost:9004 localhost:9005 localhost:9006

# 4. 100,000개 테스트 데이터 삽입
./redisctl --password myredispassword populate-test-data --num-keys 100000 localhost:9001

# 5. 노드 추가 (9007을 마스터로)
./redisctl --password myredispassword add-node localhost:9007 localhost:9001

# 6. 노드 추가 (9008을 9007의 복제본으로)
# 먼저 9007의 노드 ID를 확인:
./redisctl --password myredispassword check localhost:9001
# 그 후 9007의 node-id를 사용:
./redisctl --password myredispassword add-node --master-id <9007-node-id> localhost:9008 localhost:9001

# 7. 리샤딩 (1000 슬롯을 9007로 이동)
./redisctl --password myredispassword reshard `
  --from <source-master-id> --to <9007-node-id> --slots 1000 localhost:9001

# 8. 자동 리밸런싱 (드라이런으로 계획 확인)
./redisctl --password myredispassword rebalance --dry-run localhost:9001

# 9. 자동 리밸런싱 실행 (불균형 해소)
./redisctl --password myredispassword rebalance localhost:9001

# 10. 리샤딩 (1000 슬롯을 9007에서 다시 이동)
./redisctl --password myredispassword reshard `
  --from <9007-node-id> --to <target-master-id> --slots 1000 localhost:9001

# 11. 노드 제거 1 - rep
./redisctl --password myredispassword del-node localhost:9001 <9008-node-id>

# 12. 노드 제거 2 - rep
./redisctl --password myredispassword del-node localhost:9001 <9007-node-id>

# 각 단계마다 상태 확인
./redisctl --password myredispassword check localhost:9001
```

## 설계

### 아키텍처

- **Cobra**: CLI 프레임워크로 명령어 구조화
- **Lipgloss**: 터미널 스타일링
- **Redis v9**: 최신 Redis Go 클라이언트 라이브러리 (`github.com/redis/go-redis/v9`)

### 인증 처리

- `--password` 플래그는 필수입니다
- `--user` 플래그는 선택사항 (기본 인증 사용시)

## 개발 정보

### 프로젝트 구조

```
redisctl/
├── main.go                 # 메인 엔트리 포인트
├── cmd/                    # 명령어 구현
│   ├── create.go          # 클러스터 생성
│   ├── commands.go        # 기타 명령어들
│   └── ...
├── internal/              # 내부 패키지
│   ├── config/           # 설정 관리
│   ├── redis/            # Redis 클라이언트 래퍼
│   └── styles/           # UI 스타일링
└── go.mod                # Go 모듈 설정
```

### 확장 가능성

- 새로운 명령어 추가 시 `cmd/` 디렉토리에 파일 추가
- Redis 클라이언트 기능 확장은 `internal/redis/` 수정
- UI 스타일 변경은 `internal/styles/` 수정
