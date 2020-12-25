# 应用名称
name = proxy
# 型号
model = ${name}
# 固件版本
# git describe --tags `git rev-list --tags --max-count=1`
version = `git describe --always --tags`
# api版本
APIversion = v0.0.1
# 设置固件名称
firmwareName = ${name}

execveFile := ${firmwareName}

# 路径相关
BinDir=${CURDIR}/bin
Release=${BinDir}/release-${version}

# 编译平台
platform = CGO_ENABLED=0
# 编译选项,如tags,多个采用','分开 sqlite3
opts = -trimpath -tags=jsoniter
# 编译flags
path = github.com/thinkgos/go-core-package/builder
flags = -ldflags "-X '${path}.BuildTime=`date "+%F %T %z"`' \
	-X '${path}.GitCommit=`git rev-parse --short=8 HEAD`' \
	-X '${path}.GitFullCommit=`git rev-parse HEAD`' \
	-X '${path}.Name=${name}' \
	-X '${path}.Model=${model}' \
	-X '${path}.Version=${version}' \
	-X '${path}.APIVersion=${APIversion}' -w" # -s 引起gops无法识别go版本号,upx压缩也同样

system:
	@echo "----> system executable building..."
	@mkdir -p ${Release} ${BinDir}
	@${platform} go build ${opts} ${flags} -o ${BinDir}/${execveFile} .
	@#upx --best --lzma ${BinDir}/${execveFile}
	@#tar zcfv "${Release}/${execveFile}.tar.gz" ${BinDir}/${execveFile}
	@echo "----> system executable build successful"

help:
	@echo " ------------- How to build ------------- "
	@echo " make         -- build target"
	@echo " make clean   -- clean build files"
	@echo " ------------- How to build ------------- "

clean:
	@echo "----> cleanning"
	@go clean
	@rm -rf ${BinDir}/*
	@echo "----> clean successful"

.PHONY: system clean help



