package datagen

type DataGen interface {
	Get(key string) []byte
}
