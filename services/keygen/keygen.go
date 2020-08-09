// 生成证书
package keygen

import (
	"fmt"
	"strings"

	"github.com/thinkgos/jocasta/lib/cert"
	"github.com/thinkgos/jocasta/lib/outil"

	"github.com/thinkgos/jocasta/services"
)

type Config struct {
	CaFilePrefix   string // ca证书文件名前缀, default empty
	CertFilePrefix string // cert和key文件名前缀, default empty
	Sign           bool   // 是否签发证书 false 表示生成root ca, true 代表根据root ca签发证书
	SignDays       int    // 签发天数, 默认365天
	CommonName     string // common name
}

type Keygen struct {
	cfg Config
}

var _ services.Service = (*Keygen)(nil)

func New(cfg Config) *Keygen {
	return &Keygen{cfg}
}

func (sf *Keygen) inspectConfig() error {
	if sf.cfg.CaFilePrefix == "" {
		return fmt.Errorf("ca file name prefix required")
	}
	if sf.cfg.Sign && sf.cfg.CertFilePrefix == "" {
		return fmt.Errorf("cert file name prefix required for sign")
	}

	if sf.cfg.CommonName == "" {
		sf.cfg.CommonName = randCommonName()
	}
	return nil
}

func (sf *Keygen) Start() error {
	if err := sf.inspectConfig(); err != nil {
		return err
	}
	config := cert.Config{
		CommonName: sf.cfg.CommonName,
		Names: cert.Names{
			Country:          randCountry(),
			Organization:     sf.cfg.CommonName,
			OrganizationUnit: sf.cfg.CommonName,
		},
		Host:   []string{sf.cfg.CommonName},
		Expire: uint64(sf.cfg.SignDays * 24),
	}
	if !sf.cfg.Sign {
		return cert.CreateCAFile(sf.cfg.CaFilePrefix, config)
	}
	caCert, caKey, err := cert.ParseCrtAndKeyFile(sf.cfg.CaFilePrefix+cert.CertFileSuffix,
		sf.cfg.CaFilePrefix+cert.KeyFileSuffix)
	if err != nil {
		return err
	}
	return cert.CreateSignFile(caCert, caKey, sf.cfg.CertFilePrefix, config)
}

func (sf *Keygen) Stop() {}

func randCommonName() string {
	domainSuffixList := []string{
		".com", ".edu", ".gov", ".int", ".mil", ".net", ".org", ".biz", ".info",
		".pro", ".name", ".museum", ".coop", ".aero", ".xxx", ".idv", ".ac",
		".ad", ".ae", ".af", ".ag", ".ai", ".al", ".am", ".an", ".ao", ".aq",
		".ar", ".as", ".at", ".au", ".aw", ".az", ".ba", ".bb", ".bd", ".be",
		".bf", ".bg", ".bh", ".bi", ".bj", ".bm", ".bn", ".bo", ".br", ".bs",
		".bt", ".bv", ".bw", ".by", ".bz", ".ca", ".cc", ".cd", ".cf", ".cg",
		".ch", ".ci", ".ck", ".cl", ".cm", ".cn", ".co", ".cr", ".cu", ".cv",
		".cx", ".cy", ".cz", ".de", ".dj", ".dk", ".dm", ".do", ".dz", ".ec",
		".ee", ".eg", ".eh", ".er", ".es", ".et", ".eu", ".fi", ".fj", ".fk",
		".fm", ".fo", ".fr", ".ga", ".gd", ".ge", ".gf", ".gg", ".gh", ".gi",
		".gl", ".gm", ".gn", ".gp", ".gq", ".gr", ".gs", ".gt", ".gu", ".gw",
		".gy", ".hk", ".hm", ".hn", ".hr", ".ht", ".hu", ".id", ".ie", ".il",
		".im", ".in", ".io", ".iq", ".ir", ".is", ".it", ".je", ".jm", ".jo",
		".jp", ".ke", ".kg", ".kh", ".ki", ".km", ".kn", ".kp", ".kr", ".kw",
		".ky", ".kz", ".la", ".lb", ".lc", ".li", ".lk", ".lr", ".ls", ".lt",
		".lu", ".lv", ".ly", ".ma", ".mc", ".md", ".mg", ".mh", ".mk", ".ml",
		".mm", ".mn", ".mo", ".mp", ".mq", ".mr", ".ms", ".mt", ".mu", ".mv",
		".mw", ".mx", ".my", ".mz", ".na", ".nc", ".ne", ".nf", ".ng", ".ni",
		".nl", ".no", ".np", ".nr", ".nu", ".nz", ".om", ".pa", ".pe", ".pf",
		".pg", ".ph", ".pk", ".pl", ".pm", ".pn", ".pr", ".ps", ".pt", ".pw",
		".py", ".qa", ".re", ".ro", ".ru", ".rw", ".sa", ".sb", ".sc", ".sd",
		".se", ".sg", ".sh", ".si", ".sj", ".sk", ".sl", ".sm", ".sn", ".so",
		".sr", ".st", ".sv", ".sy", ".sz", ".tc", ".td", ".tf", ".tg", ".th",
		".tj", ".tk", ".tl", ".tm", ".tn", ".to", ".tp", ".tr", ".tt", ".tv",
		".tw", ".tz", ".ua", ".ug", ".uk", ".um", ".us", ".uy", ".uz", ".va",
		".vc", ".ve", ".vg", ".vi", ".vn", ".vu", ".wf", ".ws", ".ye", ".yt",
		".yu", ".yr", ".za", ".zm", ".zw",
	}
	return strings.ToLower(outil.RandString(int(outil.RandInt(4)%10)) +
		domainSuffixList[int(outil.RandInt(4))%len(domainSuffixList)])
}

func randCountry() string {
	countryList := []string{
		"AD", "AE", "AF", "AG", "AI", "AL", "AM", "AO", "AR", "AT", "AU", "AZ",
		"BB", "BD", "BE", "BF", "BG", "BH", "BI", "BJ", "BL", "BM", "BN", "BO",
		"BR", "BS", "BW", "BY", "BZ", "CA", "CF", "CG", "CH", "CK", "CL", "CM",
		"CN", "CO", "CR", "CS", "CU", "CY", "CZ", "DE", "DJ", "DK", "DO", "DZ",
		"EC", "EE", "EG", "ES", "ET", "FI", "FJ", "FR", "GA", "GB", "GD", "GE",
		"GF", "GH", "GI", "GM", "GN", "GR", "GT", "GU", "GY", "HK", "HN", "HT",
		"HU", "ID", "IE", "IL", "IN", "IQ", "IR", "IS", "IT", "JM", "JO", "JP",
		"KE", "KG", "KH", "KP", "KR", "KT", "KW", "KZ", "LA", "LB", "LC", "LI",
		"LK", "LR", "LS", "LT", "LU", "LV", "LY", "MA", "MC", "MD", "MG", "ML",
		"MM", "MN", "MO", "MS", "MT", "MU", "MV", "MW", "MX", "MY", "MZ", "NA",
		"NE", "NG", "NI", "NL", "NO", "NP", "NR", "NZ", "OM", "PA", "PE", "PF",
		"PG", "PH", "PK", "PL", "PR", "PT", "PY", "QA", "RO", "RU", "SA", "SB",
		"SC", "SD", "SE", "SG", "SI", "SK", "SL", "SM", "SN", "SO", "SR", "ST",
		"SV", "SY", "SZ", "TD", "TG", "TH", "TJ", "TM", "TN", "TO", "TR", "TT",
		"TW", "TZ", "UA", "UG", "US", "UY", "UZ", "VC", "VE", "VN", "YE", "YU",
		"ZA", "ZM", "ZR", "ZW",
	}
	return countryList[int(outil.RandInt(4))%len(countryList)]
}
