package setting

type HttpConfig struct {
	Port uint
}

type ServerConfig struct {
	Http    HttpConfig
	Prefork bool
}

type AppConfig struct {
	Debug bool
}

type DatabaseConfig struct {
	DBType       string
	UserName     string
	Password     string
	Host         string
	DBName       string
	Charset      string
	TablePrefix  string
	ParseTime    bool
	MaxIdleConns int
	MaxOpenConns int
}
