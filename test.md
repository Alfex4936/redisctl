과제에서 요구하는 시나리오 테스트를 위한 모든 redisctl 명령어를 단계별로 보여드리겠습니다. ## 🧪 **Redis 클러스터 시나리오 테스트 - 전체 명령어 가이드**

### **📋 시나리오 개요**
1. 8개 Redis 프로세스 준비 (포트 9001~9008)
2. 6개 노드로 클러스터 생성 (3 마스터 + 3 복제본)
3. 100,000개 테스트 데이터 삽입
4. 새 노드 추가 (9007 마스터, 9008 복제본)
5. 리샤딩 테스트 (1000 슬롯 이동 후 되돌리기)
6. 노드 제거 (9007, 9008)

---

### **🎯 1단계: Redis 프로세스 준비**

```bash
# Linux/macOS에서 설정 스크립트 실행
./setup-redis-cluster.sh

# 각 터미널 창에서 Redis 서버 시작 (8개 창 필요)
cd redis-nodes/9001 && redis-server redis.conf  # 터미널 1
cd redis-nodes/9002 && redis-server redis.conf  # 터미널 2
cd redis-nodes/9003 && redis-server redis.conf  # 터미널 3
cd redis-nodes/9004 && redis-server redis.conf  # 터미널 4
cd redis-nodes/9005 && redis-server redis.conf  # 터미널 5
cd redis-nodes/9006 && redis-server redis.conf  # 터미널 6
cd redis-nodes/9007 && redis-server redis.conf  # 터미널 7
cd redis-nodes/9008 && redis-server redis.conf  # 터미널 8
```

---

### **🔧 2단계: 클러스터 생성 (3 마스터 + 3 복제본)**

```bash
# 6개 노드로 클러스터 생성 (3 마스터, 각각 1개 복제본)
./redisctl --password myredispassword create --replicas 1 \
  localhost:9001 localhost:9002 localhost:9003 \
  localhost:9004 localhost:9005 localhost:9006
```

**상태 확인:**
```bash
./redisctl --password myredispassword check localhost:9001
```

---

### **🧪 3단계: 테스트 데이터 삽입 (100,000개 키)**

```bash
# 100,000개 테스트 데이터 삽입
./redisctl --password myredispassword populate-test-data --num-keys 100000 localhost:9001
```

**상태 확인:**
```bash
./redisctl --password myredispassword check localhost:9001
```

---

### **➕ 4단계: 새 노드 추가**

#### **4-1. 9007을 마스터로 추가**
```bash
# 9007을 새 마스터로 추가 (슬롯 없음)
./redisctl --password myredispassword add-node localhost:9007 localhost:9001
```

#### **4-2. 9007의 노드 ID 확인**
```bash
# 9007의 노드 ID를 확인하여 복사
./redisctl --password myredispassword check localhost:9001
```

#### **4-3. 9008을 9007의 복제본으로 추가**
```bash
# 9007의 노드 ID를 사용하여 9008을 복제본으로 추가
./redisctl --password myredispassword add-node --master-id <9007-node-id> localhost:9008 localhost:9001
```

**상태 확인:**
```bash
./redisctl --password myredispassword check localhost:9001 --verbose
```

---

### **🔄 5단계: 리샤딩 테스트**

#### **5-1. 마스터 노드 ID 확인**
```bash
# 모든 마스터 노드 ID 확인
./redisctl --password myredispassword check --verbose localhost:9001
```

#### **5-2. 1000 슬롯을 9007로 이동**
```bash
# 기존 마스터에서 9007로 1000 슬롯 이동
./redisctl --password myredispassword reshard \
  --from <source-master-id> \
  --to <9007-node-id> \
  --slots 1000 \
  localhost:9001
```

**상태 확인:**
```bash
./redisctl --password myredispassword check localhost:9001
```

#### **5-3. 1000 슬롯을 9007에서 다시 이동**
```bash
# 9007에서 다른 마스터로 1000 슬롯 되돌리기
./redisctl --password myredispassword reshard \
  --from <9007-node-id> \
  --to <target-master-id> \
  --slots 1000 \
  localhost:9001
```

**상태 확인:**
```bash
./redisctl --password myredispassword check localhost:9001
```

---

### **➖ 6단계: 노드 제거**

#### **6-1. 9008 복제본 제거**
```bash
# 먼저 복제본(9008) 제거
./redisctl --password myredispassword del-node localhost:9001 <9008-node-id>
```

#### **6-2. 9007 마스터 제거**
```bash
# 마스터(9007) 제거 - 슬롯 자동 재분배
./redisctl --password myredispassword del-node localhost:9001 <9007-node-id>
```

**최종 상태 확인:**
```bash
./redisctl --password myredispassword check localhost:9001
```

---

### **🔍 추가 확인 명령어들**

#### **상세 클러스터 정보**
```bash
# 상세한 노드 정보 표시
./redisctl --password myredispassword check --verbose localhost:9001
```

#### **원시 클러스터 데이터**
```bash
# 원시 cluster nodes 출력
./redisctl --password myredispassword check --raw localhost:9001
```

#### **리밸런싱 (선택사항)**
```bash
# 드라이런으로 리밸런싱 계획 확인
./redisctl --password myredispassword rebalance --dry-run localhost:9001

# 실제 리밸런싱 실행
./redisctl --password myredispassword rebalance localhost:9001
```

---

### **📝 실행 순서 요약**

1. **Redis 서버 8개 시작**
2. `create --replicas 1` (6개 노드)
3. `populate-test-data --num-keys 100000`
4. `add-node` (9007 마스터)
5. `add-node --master-id` (9008 복제본)
6. `reshard` (1000 슬롯 → 9007)
7. `reshard` (1000 슬롯 ← 9007)
8. `del-node` (9008 복제본)
9. `del-node` (9007 마스터)
10. **각 단계마다 `check` 명령으로 상태 확인**

이 시나리오를 통해 Redis 클러스터의 모든 주요 관리 기능을 테스트할 수 있습니다! 🎉
