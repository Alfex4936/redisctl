#!/bin/bash

echo "Redis 클러스터 노드 시작 중..."

# 포트 범위 설정
START_PORT=9001
END_PORT=9008

# Redis 설정 디렉토리 생성
mkdir -p redis-nodes

# 각 포트에 대해 Redis 인스턴스 생성
for port in $(seq $START_PORT $END_PORT); do
    echo "노드 $port 설정 중..."
    
    # 포트별 디렉토리 생성
    mkdir -p redis-nodes/$port
    
    # 설정 파일 생성
    cat > redis-nodes/$port/redis.conf << EOF
# Redis Cluster Node $port Configuration
port $port
bind 127.0.0.1

# 클러스터 설정
cluster-enabled yes
cluster-config-file nodes-$port.conf
cluster-node-timeout 5000

# 인증 설정
requirepass myredispassword
masterauth myredispassword

# 데이터 저장
appendonly yes
appendfilename "appendonly-$port.aof"
dbfilename "dump-$port.rdb"

# 로그 설정
loglevel notice
logfile "redis-$port.log"

# 작업 디렉토리
dir ./

# 성능 설정
tcp-keepalive 300
timeout 0
save 900 1
save 300 10
save 60 10000
EOF

    echo "✅ 노드 $port 설정 완료"
done

echo ""
echo "Redis 노드들을 시작하려면 다음 명령어들을 각각 실행하세요:"
echo ""

for port in $(seq $START_PORT $END_PORT); do
    echo "# 터미널 $((port - START_PORT + 1)): 포트 $port"
    echo "cd redis-nodes/$port && redis-server redis.conf"
    echo ""
done

echo "모든 노드가 시작된 후, 클러스터를 생성하세요:"
echo ""
echo "# 6개 노드로 클러스터 생성 (3 마스터 + 3 복제본)"
echo "./redisctl --password myredispassword create --replicas 1 \\"
echo "  localhost:9001 localhost:9002 localhost:9003 \\"
echo "  localhost:9004 localhost:9005 localhost:9006"
echo ""
echo "# 100,000개 테스트 데이터 삽입"
echo "./redisctl --password myredispassword populate-test-data --num-keys 100000 localhost:9001"
echo ""
echo "# 추가 노드들 추가"
echo "./redisctl --password myredispassword add-node localhost:9007 localhost:9001"
echo "./redisctl --password myredispassword add-node --master-id <9007-node-id> localhost:9008 localhost:9001"
