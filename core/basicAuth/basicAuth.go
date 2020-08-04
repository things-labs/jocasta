package basicAuth

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	cmap "github.com/orcaman/concurrent-map"

	"github.com/thinkgos/jocasta/core/idns"
)

// Center basic auth center
type Center struct {
	passwords   cmap.ConcurrentMap // 用户名 -> 密码 映射
	dns         *idns.Resolver     // 可选dns解析服务
	url         string             // 可选上级授权中心
	successCode int                // http请求成功码
	timeout     time.Duration      // http请求超时时间,单位ms
	retry       uint               // 重试次数

}

// New a Center with option
// basic auth 认证中心
// 可选DNS服务器
// 可选第三方basic auth 中心认证服务. 需设置正确的url, 超时时间, 成功码, 重试次数.
// 采用方法: GET
// URL格式: url + ["&"] +"user=username&pass=pwd&ip=userIP&local_ip=localIP&target=target"
// 其它参数建议:
// successCode: 204
// 超时时间: 1s
// 重试次数: 3
func New(opts ...Option) *Center {
	center := &Center{passwords: cmap.New()}
	for _, opt := range opts {
		opt(center)
	}
	return center
}

// SetDNSServer 设置DNS服务器,用于解析url
func (sf *Center) SetDNSServer(dns *idns.Resolver) *Center {
	sf.dns = dns
	return sf
}

// SetAuthURL 设置第三方basic auth 中心认证服务. url, 超时时间, 成功码, 重试次数.
func (sf *Center) SetAuthURL(url string, timeout time.Duration, code int, retry uint) *Center {
	sf.url = url
	sf.successCode = code
	sf.timeout = timeout
	sf.retry = retry
	return sf
}

// LoadFromFile 从文件加载用户-密码对,返回加载成功的数目
// 一行一条,格式 user:password , # 为注释
func (sf *Center) LoadFromFile(filename string) (n int, err error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	userPasswords := strings.Split(strings.Replace(string(content), "\r", "", -1), "\n")
	for _, up := range userPasswords {
		up = strings.Trim(up, " ")
		if up == "" || strings.HasPrefix(up, "#") { // 忽略注释
			continue
		}
		n += sf.Add(up)
	}
	return
}

// Add 增加用户密码对,格式user:password
func (sf *Center) Add(userPwdPair ...string) (n int) {
	for _, up := range userPwdPair {
		u := strings.Split(up, ":")
		if len(u) == 2 {
			sf.passwords.Set(u[0], u[1])
			n++
		}
	}
	return
}

// Delete 删除用户
func (sf *Center) Delete(users ...string) {
	for _, u := range users {
		sf.passwords.Remove(u)
	}
}

// Total 用户总数
func (sf *Center) Total() int {
	return sf.passwords.Count()
}

// Has 是否存在对应用户
func (sf *Center) Has(user string) bool {
	return sf.passwords.Has(user)
}

// Verify verify from local center,if url has set, it will verify from basic server
func (sf *Center) Verify(userPwdPair string, userIP, localIP, target string) bool {
	strs := strings.Split(strings.Trim(userPwdPair, " "), ":")
	if len(strs) != 2 {
		return false
	}
	user, pwd := strs[0], strs[1]
	if ok := sf.VerifyFromLocal(user, pwd); ok {
		return true
	}
	return sf.VerifyFromURL(user, pwd, userIP, localIP, target) == nil
}

// VerifyFromLocal 校验对应用户密码和账号
func (sf *Center) VerifyFromLocal(user, pwd string) bool {
	if p, found := sf.passwords.Get(user); found {
		return p.(string) == pwd
	}
	return false
}

// VerifyFromURL verify only from url basic server
func (sf *Center) VerifyFromURL(user, pwd, userIP, localIP, target string) (err error) {
	if sf.url == "" {
		return errors.New("invalid url")
	}

	URL := sf.url
	if strings.Contains(URL, "?") {
		URL += "&"
	} else {
		URL += "?"
	}
	URL += fmt.Sprintf("user=%s&pass=%s&ip=%s&local_ip=%s&target=%s",
		user, pwd, userIP, localIP, url.QueryEscape(target))
	getURL := URL
	var domain string
	if sf.dns != nil {
		_url, _ := url.Parse(sf.url)
		domain = _url.Host
		domainIP := sf.dns.MustResolve(domain)
		if domainIP != domain {
			getURL = strings.Replace(URL, domain, domainIP, 1)
		}
	}

	return backoff.Retry(func() error {
		body, code, err := httpGet(getURL, sf.timeout, domain)
		if err != nil {
			return fmt.Errorf("auth fail from url %s,resonse %s , %s -> %s", URL, err, userIP, localIP)
		}
		if code == sf.successCode {
			return nil
		}
		if len(body) > 50 {
			body = body[:50]
		}
		return fmt.Errorf("auth fail from url %s,resonse code: %d, except: %d , %s -> %s, %s",
			URL, code, sf.successCode, userIP, localIP, string(body))
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), uint64(sf.retry)))
}

// httpGet http get请求
func httpGet(url string, timeout time.Duration, host ...string) (body []byte, code int, err error) {
	tr := &http.Transport{}
	if strings.Contains(url, "https://") {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	defer tr.CloseIdleConnections()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	if len(host) > 0 && host[0] != "" {
		req.Host = host[0]
	}

	client := &http.Client{Timeout: timeout, Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	code = resp.StatusCode
	body, err = ioutil.ReadAll(resp.Body)
	return
}

// Format user pwd --> user:pwd
func Format(user, pwd string) string {
	return user + ":" + pwd
}
