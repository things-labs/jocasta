
# 型号
model = cproxy
# 固件版本
version = v0.0.1
# api版本
APIversion = v0.0.1
# 设置固件名称
firmwareName = cproxy

execveFile := ${firmwareName}

# 路径相关
BINDIR=${CURDIR}/bin
RELEASE=${BINDIR}/release-${version}

# 编译平台
platform = CGO_ENABLED=0
# 编译tags,多个采用','分开
opts =  -trimpath
# 编译flags
path = github.com/thinkgos/ppcore/builder
flags = -ldflags "-X '${path}.BuildTime=`date "+%F %T %z"`' \
	-X '${path}.GitCommit=`git rev-parse --short=8 HEAD`' \
	-X '${path}.GitFullCommit=`git rev-parse HEAD`' \
	-X '${path}.Version=${version}' \
	-X '${path}.Model=${model}' \
	-X '${path}.APIVersion=${APIversion}' -s -w"

target:
	@echo "----> system executable building..."
	@mkdir -p ${RELEASE}
	@mkdir -p ${BINDIR}
	@${platform} go build ${opts} ${flags} -o ${execveFile} . && \
	upx -9 ${execveFile} && \
	tar zcfv "${RELEASE}/${execveFile}.tar.gz" ${execveFile}
	@echo "----> system executable build successful"

help:
	@echo " ------------- How to build ------------- "
	@echo " make         -- build target"
	@echo " make clean   -- clean build files"
	@echo " ------------- How to build ------------- "

clean:
	@echo "----> cleanning"
	@go clean
	@rm -rf ${BINDIR}/*
	@echo "----> clean successful"

.PHONY: target clean help



