```bash
 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  redis-cli -p 9001 -a myredispassword cluster 
nodes
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
cbf97663d8c23dd11d491253323e87bcc315e7ea 127.0.0.1:9005@19005 slave 27d4ef4e6f7800e9fa3fa5d9f015d8c577b3fc51 0 1751545330000 3 connected
313b74b4d6796279657f038b3cccea5b44e44403 127.0.0.1:9004@19004 slave f58b10afadc40fedfd6a8b4bb45eea79d0827df8 0 1751545330092 0 connected
c0073ec0ba5f6c7b35e4caa9a7c166425bdd75ac 127.0.0.1:9003@19003 master - 0 1751545331095 1 connected 10923-16383
f58b10afadc40fedfd6a8b4bb45eea79d0827df8 127.0.0.1:9001@19001 myself,master - 0 1751545330000 0 connected 0-5461
4c781f33a9792d81dd734e9b0ca3fb5f481a94c0 127.0.0.1:9006@19006 slave c0073ec0ba5f6c7b35e4caa9a7c166425bdd75ac 0 1751545331000 1 connected
27d4ef4e6f7800e9fa3fa5d9f015d8c577b3fc51 127.0.0.1:9002@19002 master - 0 1751545330594 3 connected 5462-10922
 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  redis-cli -p 9001 -a myredispassword cluster 
info
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
cluster_state:ok
cluster_slots_assigned:16384
cluster_slots_ok:16384
cluster_slots_pfail:0
cluster_slots_fail:0
cluster_known_nodes:6
cluster_size:3
cluster_current_epoch:5
cluster_my_epoch:0
cluster_stats_messages_ping_sent:83
cluster_stats_messages_pong_sent:94
cluster_stats_messages_meet_sent:5
cluster_stats_messages_sent:182
cluster_stats_messages_ping_received:94
cluster_stats_messages_pong_received:88
cluster_stats_messages_received:182
total_cluster_links_buffer_limit_exceeded:0

 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  redis-cli -c -p 9001 -a myredispassword set t
est_key "Hello Redis Cluster"
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
OK
 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  redis-cli -c -p 9002 -a myredispassword get t
est_key
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
"Hello Redis Cluster"

 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  redis-cli -p 9001 -a myredispas
sword cluster nodes
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
cbf97663d8c23dd11d491253323e87bcc315e7ea 127.0.0.1:9005@19005 slave 27d4ef4e6f7800e9fa3fa5d9f015d8c577b3fc51 0 1751546310549 3 connected
07a152a715fc916d3e19932ebeb758721899ef71 127.0.0.1:9007@19007 master - 0 1751546310000 9 connected 5444-5461
f58b10afadc40fedfd6a8b4bb45eea79d0827df8 127.0.0.1:9001@19001 myself,master - 0 1751546309000 8 connected 0-5443 [5443->-07a152a715fc916d3e19932ebeb758721899ef71]
27d4ef4e6f7800e9fa3fa5d9f015d8c577b3fc51 127.0.0.1:9002@19002 master - 0 1751546308541 3 connected 5462-10922
313b74b4d6796279657f038b3cccea5b44e44403 127.0.0.1:9004@19004 slave f58b10afadc40fedfd6a8b4bb45eea79d0827df8 0 1751546310549 8 connected
694282d2c68d80aeb302187d98401f4b192d23d2 127.0.0.1:9008@19008 slave 07a152a715fc916d3e19932ebeb758721899ef71 0 1751546309000 9 connected
c0073ec0ba5f6c7b35e4caa9a7c166425bdd75ac 127.0.0.1:9003@19003 master - 0 1751546310000 1 connected 10923-16383
f9290dba706f834bdb16365e39f5c832c38fceac 127.0.0.1:9009@19009 slave 07a152a715fc916d3e19932ebeb758721899ef71 0 1751546309546 9 connected
4c781f33a9792d81dd734e9b0ca3fb5f481a94c0 127.0.0.1:9006@19006 slave c0073ec0ba5f6c7b35e4caa9a7c166425bdd75ac 0 1751546310549 1 connected
 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  redis-cli -p 9001 -a myredispassw
ord cluster setslot 5443 stable
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
OK

 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  redis-cli -p 9007 -a myredispassw
ord cluster setslot 5443 stable
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
OK
 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  redis-cli -p 9001 -a myredispassw
ord cluster nodes
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
cbf97663d8c23dd11d491253323e87bcc315e7ea 127.0.0.1:9005@19005 slave 27d4ef4e6f7800e9fa3fa5d9f015d8c577b3fc51 0 1751546359525 3 connected
07a152a715fc916d3e19932ebeb758721899ef71 127.0.0.1:9007@19007 master - 0 1751546360530 9 connected 5444-5461
f58b10afadc40fedfd6a8b4bb45eea79d0827df8 127.0.0.1:9001@19001 myself,master - 0 1751546359000 8 connected 0-5443
27d4ef4e6f7800e9fa3fa5d9f015d8c577b3fc51 127.0.0.1:9002@19002 master - 0 1751546360530 3 connected 5462-10922
313b74b4d6796279657f038b3cccea5b44e44403 127.0.0.1:9004@19004 slave f58b10afadc40fedfd6a8b4bb45eea79d0827df8 0 1751546360730 8 connected
694282d2c68d80aeb302187d98401f4b192d23d2 127.0.0.1:9008@19008 slave 07a152a715fc916d3e19932ebeb758721899ef71 0 1751546359000 9 connected
c0073ec0ba5f6c7b35e4caa9a7c166425bdd75ac 127.0.0.1:9003@19003 master - 0 1751546359000 1 connected 10923-16383
f9290dba706f834bdb16365e39f5c832c38fceac 127.0.0.1:9009@19009 slave 07a152a715fc916d3e19932ebeb758721899ef71 0 1751546359726 9 connected
4c781f33a9792d81dd734e9b0ca3fb5f481a94c0 127.0.0.1:9006@19006 slave c0073ec0ba5f6c7b35e4caa9a7c166425bdd75ac 0 1751546360000 1 connected

```

문제를 분석해보니 "ERR Slot 0 is already busy" 오류는 Redis 클러스터 프로토콜을 올바르게 따르지 않아서 발생하는 것입니다. 

현재 코드는 슬롯 마이그레이션 후에 `CLUSTER SETSLOT slot STABLE`을 호출한 후, 다시 `ClusterAddSlots`를 호출하고 있습니다. 하지만 Redis 클러스터에서는 마이그레이션 완료 후 `CLUSTER SETSLOT slot NODE target-node-id`를 사용하여 슬롯 소유권을 이전해야 합니다.




명령: ./redisctl --password myredispassword rebalance localhost:9001

오류 메시지: ERR Can't assign hashslot 16368 to a different node while I still hold keys for this hash slot.

원인: 이 오류는 Redis 클러스터에서 슬롯의 소유권을 다른 노드로 이전하려고 할 때, 아직 해당 슬롯에 키가 남아있는 경우 발생합니다. 슬롯을 이전하는 올바른 절차는 다음과 같습니다.

소스 노드에서 대상 노드로 슬롯의 모든 키를 이전합니다.
키 이전이 완료된 후, 클러스터의 모든 노드에 슬롯의 소유권이 변경되었음을 알립니다.
현재 redisctl 도구는 키를 이전하는 과정(1번)을 완료하기 전에 소유권 이전(2번)을 시도하여 오류가 발생하는 것으로 보입니다. rebalance 또는 reshard 명령어의 로직에 문제가 있을 가능성이 높습니다.

🔧 수정된 내용
문제점
원인: 슬롯 이전 시 ClusterGetKeysInSlot을 한 번만 호출해서 일부 키만 이전하고, 아직 키가 남아있는 상태에서 슬롯 소유권을 변경하려고 시도
오류: ERR Can't assign hashslot 16368 to a different node while I still hold keys for this hash slot.
해결 방법
1. reshard.go의 migrateSlot 함수 수정:

2. rebalance.go의 reshardSlots 함수 수정:

동일한 로직으로 슬롯에 키가 완전히 없어질 때까지 반복적으로 키를 확인하고 이전
장점
✅ 완전한 키 이전 보장: 슬롯에 키가 없을 때까지 반복하므로 모든 키가 확실히 이전됩니다
✅ 동시성 처리: 이전 중에 새로 추가되는 키도 처리됩니다
✅ 안전한 소유권 변경: 키가 모두 이전된 후에만 슬롯 소유권을 변경합니다
✅ 파이프라인 효율성: 기존 파이프라인 크기 설정을 유지하면서도 안전성을 확보합니다

## ✅ 2025-07-03: Redis 클러스터 리밸런싱 성공 사례

### 🔧 문제 해결: hashslot assignment 오류 수정

**문제상황:**
```
ERR Can't assign hashslot 16368 to a different node while I still hold keys for this hash slot.
```

**원인 분석:**
- `ClusterGetKeysInSlot`을 한 번만 호출하여 일부 키만 이전
- 키가 남아있는 상태에서 슬롯 소유권 변경 시도

**해결책:**
- `reshard.go`와 `rebalance.go`의 키 이전 로직을 무한 루프로 수정
- 슬롯에 키가 완전히 없어질 때까지 반복적으로 확인

### 🎯 성공적인 리밸런싱 실행

```bash
./redisctl --password myredispassword rebalance localhost:9001
⚖️ Redis 클러스터 슬롯 균형 조정
클러스터: localhost:9001 

1. 클러스터 연결 확인... 완료
2. 현재 슬롯 분배 조회... 완료
현재 불균형도: 98.8% 

📋 리밸런싱 계획
현재 슬롯 분배:
  127.0.0.1:9002: 4096 슬롯
  127.0.0.1:9003: 4096 슬롯
  127.0.0.1:9001: 4096 슬롯
  127.0.0.1:9007: 4096 슬롯

이동 계획:
  1. 127.0.0.1:9002 → 127.0.0.1:9007: 1358 슬롯
  2. 127.0.0.1:9003 → 127.0.0.1:9007: 1350 슬롯
  3. 127.0.0.1:9001 → 127.0.0.1:9007: 1338 슬롯

총 이동할 슬롯: 4046 

3. 리밸런싱 실행 중...
  1/3 단계: 1358개 슬롯 이동 중...
    ✅ 완료: 1358개 슬롯 이동됨
  2/3 단계: 1350개 슬롯 이동 중...
    ✅ 완료: 1350개 슬롯 이동됨
  3/3 단계: 1338개 슬롯 이동 중...
    ✅ 완료: 1338개 슬롯 이동됨

✅ 클러스터 리밸런싱이 완료되었습니다!
최종 불균형도: 0.1%
```

### 📊 리밸런싱 후 클러스터 상태 확인

```bash
./redisctl --password myredispassword check localhost:9001
🔍 Redis 클러스터 상태 확인
클러스터: localhost:9001 

📊 클러스터 개요
총 노드: 9 
마스터: 4, 레플리카: 5
슬롯 커버리지: 16384 / 16384 (100.0%)
예상 키 수: 91.8K 
클러스터 상태: ok
클러스터 크기: 4 
현재 에포크: 11 

🔗 노드 상세
  마스터 127.0.0.1:9001 | f58b10af... | 슬롯: 4096 (10-4105)
  마스터 127.0.0.1:9002 | 27d4ef4e... | 슬롯: 4096 (5462-9557)
  마스터 127.0.0.1:9003 | c0073ec0... | 슬롯: 4096 (10923-15018)
  마스터 127.0.0.1:9007 | 07a152a7... | 슬롯: 4096 
  레플리카 127.0.0.1:9004 | 313b74b4... | 마스터: f58b10af...
  레플리카 127.0.0.1:9005 | cbf97663... | 마스터: 27d4ef4e...
  레플리카 127.0.0.1:9006 | 4c781f33... | 마스터: c0073ec0...
  레플리카 127.0.0.1:9008 | 694282d2... | 마스터: 07a152a7...
  레플리카 127.0.0.1:9009 | f9290dba... | 마스터: 07a152a7...

🏥 건강성 보고서
✅ 모든 건강성 검사 통과
✅ 클러스터가 정상 상태입니다!
```

### 🎯 개선 효과

| 항목 | 수정 전 | 수정 후 |
|------|---------|---------|
| **불균형도** | 98.8% | 0.1% |
| **오류 발생** | hashslot assignment 오류 | 오류 없음 |
| **키 이전 안전성** | 불완전 | 완전 보장 |
| **동시성 처리** | 미지원 | 지원 |

### 🔧 기술적 개선사항

**1. 완전한 키 이전 보장:**
- 슬롯에 키가 없을 때까지 반복적으로 확인
- 이전 중 새로 추가되는 키도 처리

**2. 안전한 소유권 변경:**
- 모든 키 이전 완료 후에만 `CLUSTER SETSLOT` 실행
- Redis 클러스터 프로토콜 준수

**3. 파이프라인 효율성 유지:**
- 기존 파이프라인 크기 설정 유지
- 성능과 안전성 동시 확보

---

📊 Redis 클러스터 불균형도 기준
✅ 프로덕션 허용 기준:
0-2%: 🟢 Excellent - 이상적인 상태
2-5%: 🟡 Good - 양호한 상태
5-10%: 🟠 Acceptable - 허용 가능
10%+: 🔴 Needs Rebalancing - 리밸런싱 필요
🎯 현재 상태 분석:
완벽한 분배 상태입니다! 모든 마스터 노드가 정확히 4096 슬롯씩 가지고 있네요.

🎯 프로덕션에서의 0.1% 불균형 평가
✅ 매우 우수한 상태 - 이유:
1. 이론적 완벽 분배:

16,384 슬롯 ÷ 4 마스터 = 4,096 슬롯/노드
현재: 모든 노드가 정확히 4,096 슬롯 보유
실질적으로 완벽한 균형
2. 성능 측면:

3. 운영 측면:

📋 프로덕션 모니터링 권장사항
🔍 정기 체크 스케줄:
⚠️ 리밸런싱 트리거 조건:
5% 이상: 리밸런싱 검토
10% 이상: 즉시 리밸런싱 실행
노드 추가/제거 후: 항상 리밸런싱

for port in 9001 9002 9003 9004 9005 9006 9007 9008; do
  echo "=== 포트 $port 데이터 삭제 ==="
  redis-cli -p $port -a myredispassword flushall
  redis-cli -p $port -a myredispassword cluster reset hard
  echo ""
done


 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  time ./redisctl --p
assword myredispassword populate-test-data --num-keys 1000000 localhost:9001
🧪 Redis 클러스터 테스트 데이터 생성
클러스터:  localhost:9001 
생성할 키 수:  1.0M 

1. 클러스터 연결 확인... 완료
2. 클러스터 상태 확인... 완료
병렬 작업자 수:  20 

3. 테스트 데이터 생성 중...
진행률:  929.9K / 1.0M  (93.0%) | 속도: 185972 keys/sec | 오류: 0

 📊 생성 결과 
총 키 수:  1.0M 
성공: 1.0M (100.0%)
소요 시간:  5.4s 
처리 속도:  184.9K  keys/sec
작업자 수: 20
성능: 우수

✅ 테스트 데이터 생성이 완료되었습니다!
./redisctl --password myredispassword populate-test-data --num-keys 1000000   9.72s user 11.62s system 391% cpu 5.446 total

좋은 지적입니다! del-node 명령에서 마스터 노드 제거 시 슬롯 재분배 로직에 문제가 있네요. 레플리카는 잘 되지만 마스터는 실패하고 있습니다.
🐛 문제점 분석:
잘못된 클라이언트 사용: 소스 노드에 SETSLOT MIGRATING 명령을 보낼 때 클러스터 클라이언트를 사용하고 있는데, 이는 소스 노드로 직접 연결되지 않을 수 있습니다.

키 이전 로직 불완전: rebalance.go에서 수정한 무한 루프 로직이 적용되지 않았습니다.



 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  ./redisctl --passwo
rd myredispassword reshard --from 203138d32bfb9f4322ba4fb50c58b6e60bb97eaf --to 62ba
c3d10587be69081a40e2c558adab2613c769 --slots 2 localhost:9001
리샤딩 시작...
클러스터 노드: localhost:9001
소스 마스터: 203138d32bfb9f4322ba4fb50c58b6e60bb97eaf
대상 마스터: 62bac3d10587be69081a40e2c558adab2613c769
이동할 슬롯: 2개
파이프라인 크기: 10
1단계: 클러스터 연결 및 검증 중...
  localhost:9001 연결 중... ✅ 연결 성공
2단계: 노드 정보 조회 및 검증 중...
  소스: 127.0.0.1:9007@19007 (4099개 슬롯)
  대상: 127.0.0.1:9001@19001 (4095개 슬롯)
3단계: 이동할 슬롯 선택 중...
  선택된 슬롯: 4096, 4097
4단계: 마이그레이션 준비 중...
5단계: 슬롯 마이그레이션 중...
  [1/2] 슬롯 4096 마이그레이션 중... ✅ 완료
  [2/2] 슬롯 4097 마이그레이션 중... ✅ 완료
6단계: 마이그레이션 검증 중...

✅ 🎉 리샤딩이 성공적으로 완료되었습니다!

╭────────────────────────────────────────────────────────────╮
│                                                            │
│  마이그레이션 요약                                         │
│  • 이동된 슬롯 수: 2개                                     │
│  • 소스 노드 (127.0.0.1:9007@19007): 4099개 → 4097개 슬롯  │
│  • 대상 노드 (127.0.0.1:9001@19001): 4095개 → 4097개 슬롯  │
│  • 파이프라인 크기: 10                                     │
│                                                            │
│                                                            │
╰────────────────────────────────────────────────────────────╯
 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  echo "=== rebalance
 데모: 전체 클러스터 자동 균형 조정 ==="
=== rebalance 데모: 전체 클러스터 자동 균형 조정 ===
 dev@dev-linux  /media/dev/Dev2/Dev/Go/redis-manager-cli-LINE  ./redisctl --passwo
rd myredispassword rebalance --dry-run localhost:9001
⚖️ Redis 클러스터 슬롯 균형 조정
클러스터:  localhost:9001 
🔍 드라이런 모드: 실제 변경 없이 계획만 표시

1. 클러스터 연결 확인... 완료
2. 현재 슬롯 분배 조회... 완료
현재 불균형도:  0.0% 
✅ 클러스터가 이미 균형잡혀 있습니다!
임계값 5% 미만이므로 리밸런싱이 필요하지 않습니다.

🎯 결론
상황	추천 명령어	이유
정밀 제어	reshard	특정 노드 간 정확한 슬롯 수 이동
전체 최적화	rebalance	모든 노드 간 자동 균형 조정
점진적 변경	reshard	단계별 안전한 이동
대규모 변경	rebalance	효율적인 전체 재구성
신규 노드	reshard → rebalance	점진적 할당 후 전체 균형


🔄 노드 제거의 비대칭성:

클러스터에서 노드를 "제거"해도
해당 노드 자체는 여전히 클러스터 정보를 기억하고 있음
💾 클러스터 상태 지속성:

Redis는 nodes.conf 파일에 클러스터 정보 저장
노드가 제거되어도 자신의 파일은 삭제되지 않음
🤝 재연결 시도:

노드를 다시 추가하려 하면 기존 클러스터 정보와 충돌


echo "=== 노드 상태별 RESET 필요성 ==="
echo "1. 정상 독립 노드 (cluster_state: fail):"
echo "   -> RESET 불필요, 바로 추가 가능"
echo ""
echo "2. 이전 클러스터 정보 보유 노드 (cluster_state: ok):"
echo "   -> RESET 필요, 자동 초기화 후 추가"
echo ""
echo "3. 현재 활성 클러스터 멤버:"
echo "   -> 먼저 del-node로 제거 후 RESET"


Possible Bugs

Node reset while adding nodes – add-node automatically issues CLUSTER RESET HARD on a new node if it already belongs to a cluster. This could wipe an existing node unintentionally:

92  newClusterInfo, err := cm.GetClusterInfo(newNode)
93  if err == nil && newClusterInfo["cluster_state"] != "fail" {
94      fmt.Printf(" %s\n", styles.RenderWarning("이미 클러스터에 참여 중"))
95      fmt.Printf("  자동 초기화 시도 중...")
96      resetErr := newClient.Do(ctx, "CLUSTER", "RESET", "HARD").Err()
97      if resetErr != nil {
98          ...
103      fmt.Printf(" %s\n", styles.RenderSuccess("자동 초기화 완료"))
104  }

Incomplete slot updates in reshard – after migrating keys, only the source and target nodes get SET SLOT NODE updates, leaving other nodes unaware of the new slot owner:

287  // Complete the slot migration by assigning the slot to the target node
288  // This should be done on all nodes, but we'll do it on both source and target
289  err = sourceClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", targetNodeID).Err()
...
294  err = targetClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", targetNodeID).Err()

Node removal may not propagate – del-node sends CLUSTER FORGET via a single cluster client call. Because the library routes commands to one node, other nodes might not receive the forget request:

435 func removeNodeFromCluster(ctx context.Context, client *redis.ClusterClient, nodeID string) error {
436     fmt.Print(styles.InfoStyle.Render("4. 클러스터에서 노드 제거..."))
437
438     // Remove node from cluster
439     if err := client.ClusterForget(ctx, nodeID).Err(); err != nil {
440         fmt.Println(styles.ErrorStyle.Render(" 실패"))
441         return err
442     }

Populate writes one key per command – the worker for populate-test-data calls SET for every key without pipelining, which could be slow with large key counts:

262      key := fmt.Sprintf("key:%010d", keyIndex)
263      value := fmt.Sprintf("val:%010d", keyIndex)
265      err := client.Set(ctx, key, value, 0).Err()


**You are absolutely NOT overthinking this!** 🎯 This is **another serious Redis Cluster consistency bug**.

## **🚨 The Problem:**

The current `del-node` implementation has a **critical flaw**:

```go
// ❌ WRONG: Only one node forgets the removed node
func removeNodeFromCluster(ctx context.Context, client *redis.ClusterClient, nodeID string) error {
    if err := client.ClusterForget(ctx, nodeID).Err(); err != nil {
        return err
    }
    // This only affects ONE node, not the entire cluster!
}
```

## **🔍 How Redis `CLUSTER FORGET` Actually Works:**

### **Critical Facts:**
- `CLUSTER FORGET` is **local-only** - affects only the receiving node
- **NO automatic propagation** to other cluster nodes  
- **Each node maintains independent cluster membership state**

### **Current Broken Scenario:**
```bash
# 6-node cluster: A, B, C, D, E, F
# Remove node F
CLUSTER FORGET F  # Sent to only Node A

Result:
- Node A: "Node F? Never heard of it" ✅
- Nodes B,C,D,E: "Node F is down/unreachable" ❌
```

## **💥 Real-World Consequences:**

### **1. Cluster Topology Inconsistency:**
- Different nodes report different cluster membership
- Some nodes think removed node still exists (just "down")
- Breaks cluster consensus

### **2. Client Routing Issues:**
- Some nodes might still redirect clients to the "removed" node
- `MOVED` redirects to non-existent nodes
- Application errors and timeouts

### **3. Validation False Positives:**
```go
// Current validation only checks ONE node's perspective
result := client.ClusterNodes(ctx)  // ❌ Incomplete view
```

## **✅ The Official Redis Solution:**

**`redis-cli --cluster del-node` sends `CLUSTER FORGET` to EVERY node:**

```go
// ✅ CORRECT: All nodes must forget the removed node
allNodes := getClusterNodes()
for _, node := range allNodes {
    if node.ID != nodeIDToRemove {  // Don't send to node being removed
        nodeClient := connectTo(node.Address)
        nodeClient.ClusterForget(ctx, nodeIDToRemove)
    }
}
```

## **🎯 Evidence:**
- **Redis documentation** confirms `CLUSTER FORGET` is local-only
- **Official redis-cli source code** sends forget to all nodes
- **Same pattern** as the slot ownership issue we just fixed in reshard

## **🔧 Bonus Issue Found:**
Your `moveSlots()` function in del-node **also has the slot ownership propagation bug** we just fixed in reshard!

You've identified **yet another legitimate cluster consistency bug** that needs fixing for production-grade Redis cluster management! 🚀