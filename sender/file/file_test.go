package file

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/qiniu/logkit/conf"
	. "github.com/qiniu/logkit/sender/config"
	. "github.com/qiniu/logkit/utils/models"
)

func TestFileSender(t *testing.T) {
	path := "TestFileSender"
	defer os.RemoveAll(path)

	// 默认情况，使用当前时间
	{
		fsender, err := NewSender(conf.MapConf{
			KeyFileSenderPath:         filepath.Join(path, "%Y%m%d-1.log"),
			KeyFileSenderMaxOpenFiles: "10",
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, fsender.(*Sender).writers.size)

		assert.NoError(t, fsender.Send([]Data{{"abc": 123}}))
		assert.Len(t, fsender.(*Sender).writers.writers, 1)

		assert.NoError(t, fsender.Close())

		datet := time.Now().Format("20060102")
		body, err := ioutil.ReadFile(filepath.Join(path, datet+"-1.log"))
		assert.NoError(t, err)
		assert.Equal(t, `[{"abc":123}]
`, string(body))
	}

	// 设置了 timestamp key 但根本没有用到
	{
		fsender, err := NewSender(conf.MapConf{
			KeyFileSenderPath:         filepath.Join(path, "%Y%m%d-2.log"),
			KeyFileSenderTimestampKey: "timestamp",
		})
		assert.NoError(t, err)
		assert.Equal(t, 10, fsender.(*Sender).writers.size)

		assert.NoError(t, fsender.Send([]Data{{"abc": 123}}))
		assert.Len(t, fsender.(*Sender).writers.writers, 1)

		assert.NoError(t, fsender.Close())

		datet := time.Now().Format("20060102")
		body, err := ioutil.ReadFile(filepath.Join(path, datet+"-2.log"))
		assert.NoError(t, err)
		assert.Equal(t, `[{"abc":123}]
`, string(body))
	}

	// 混合 timestamp key 出现和没出现的情况，并自动清理过期的文件句柄
	{
		fsender, err := NewSender(conf.MapConf{
			KeyFileSenderPath:         filepath.Join(path, "%Y%m%d-3.log"),
			KeyFileSenderTimestampKey: "timestamp",
			KeyFileSenderMaxOpenFiles: "2",
		})
		assert.NoError(t, err)
		assert.Equal(t, 2, fsender.(*Sender).writers.size)

		// 首次写入内容，相同文件的应该写在同一行
		assert.NoError(t, fsender.Send([]Data{
			{"abc": 123},
			{"def": 456, "timestamp": "2018-07-08T15:04:05Z"},
			{"abc": 789},
			{"abc": 135},
		}))
		assert.Len(t, fsender.(*Sender).writers.writers, 2)

		time.Sleep(1 * time.Second)
		// 新开一个文件，应当有一个句柄被自动清理
		assert.NoError(t, fsender.Send([]Data{
			{"def": 456, "timestamp": "2018-07-07T15:04:05Z"},
		}))
		assert.Len(t, fsender.(*Sender).writers.writers, 2)

		// 二次追加内容，应该添加到新行
		assert.NoError(t, fsender.Send([]Data{
			{"abc": 123},
		}))

		assert.NoError(t, fsender.Close())

		datet := time.Now().Format("20060102")
		body, err := ioutil.ReadFile(filepath.Join(path, datet+"-3.log"))
		assert.NoError(t, err)
		assert.Equal(t, `[{"abc":123},{"abc":789},{"abc":135}]
[{"abc":123}]
`, string(body))

		body, err = ioutil.ReadFile(filepath.Join(path, "20180708-3.log"))
		assert.NoError(t, err)
		assert.True(t, strings.Contains(string(body), `"def":456`))

		body, err = ioutil.ReadFile(filepath.Join(path, "20180707-3.log"))
		assert.NoError(t, err)
		assert.True(t, strings.Contains(string(body), `"def":456`))
	}

	// 默认情况，使用当前时间，partition 为 2, rawWrite 为 false
	{
		fsender, err := NewSender(conf.MapConf{
			KeyFileSenderPath:         filepath.Join(path, "%Y%m%d-1.log"),
			KeyFileSenderMaxOpenFiles: "10",
			KeyFilePartition:          "2",
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, fsender.(*Sender).writers.size)

		assert.NoError(t, fsender.Send([]Data{{"abc": 123}, {"def": 456}, {"ghi": 789}, {"jkl": 120}}))
		assert.Len(t, fsender.(*Sender).writers.writers, 2)
		assert.NoError(t, fsender.Close())

		datet := time.Now().Format("20060102")
		partition0 := filepath.Join(path, "partition0")
		body, err := ioutil.ReadFile(filepath.Join(partition0, datet+"-1.log"))
		assert.NoError(t, err)
		assert.Equal(t, "[{\"abc\":123},{\"ghi\":789}]\n", string(body))

		partition1 := filepath.Join(path, "partition1")
		body, err = ioutil.ReadFile(filepath.Join(partition1, datet+"-1.log"))
		assert.NoError(t, err)
		assert.Equal(t, "[{\"def\":456},{\"jkl\":120}]\n", string(body))
	}
	os.RemoveAll(path)

	// 默认情况，使用当前时间，partition 为 2, rawWrite 为 true
	{
		fsender, err := NewSender(conf.MapConf{
			KeyFileSenderPath:         filepath.Join(path, "%Y%m%d-1.log"),
			KeyFileSenderMaxOpenFiles: "10",
			KeyFilePartition:          "2",
			KeyFileWriteRaw:           "true",
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, fsender.(*Sender).writers.size)

		assert.NoError(t, fsender.Send([]Data{{"abc": 123, "raw": "12"}, {"def": 456, "raw": "1"}, {"ghi": 789}, {"jkl": 102}}))
		assert.Len(t, fsender.(*Sender).writers.writers, 2)
		assert.NoError(t, fsender.Close())

		datet := time.Now().Format("20060102")
		partition0 := filepath.Join(path, "partition0")
		body, err := ioutil.ReadFile(filepath.Join(partition0, datet+"-1.log"))
		assert.NoError(t, err)
		assert.Equal(t, "12\n", string(body))

		partition1 := filepath.Join(path, "partition1")
		body, err = ioutil.ReadFile(filepath.Join(partition1, datet+"-1.log"))
		assert.NoError(t, err)
		assert.Equal(t, "1\n", string(body))
	}
}
