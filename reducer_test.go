package gonx

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestReadAllReducer(t *testing.T) {
	reducer := new(ReadAll)
	assert.Implements(t, (*Reducer)(nil), reducer)

	// Prepare import channel
	input := make(chan *Entry, 1)
	entry := NewEmptyEntry()
	input <- entry
	close(input)

	output := make(chan *Entry, 1) // Make it buffered to avoid deadlock
	reducer.Reduce(input, output)

	// ReadAll reducer writes input channel to the output
	result, ok := <-output
	assert.True(t, ok)
	assert.Equal(t, result, entry)
}

func TestCountReducer(t *testing.T) {
	reducer := new(Count)
	assert.Implements(t, (*Reducer)(nil), reducer)

	// Prepare import channel
	input := make(chan *Entry, 2)
	input <- NewEmptyEntry()
	input <- NewEmptyEntry()
	close(input)

	output := make(chan *Entry, 1) // Make it buffered to avoid deadlock
	reducer.Reduce(input, output)

	result, ok := <-output
	assert.True(t, ok)
	count, err := result.Field("count")
	assert.NoError(t, err)
	assert.Equal(t, count, "2")
}

func TestSumReducer(t *testing.T) {
	reducer := &Sum{[]string{"foo", "bar"}}
	assert.Implements(t, (*Reducer)(nil), reducer)

	// Prepare import channel
	input := make(chan *Entry, 2)
	input <- NewEntry(Fields{
		"uri": "/asd/fgh",
		"foo": "123",
		"bar": "234",
		"baz": "345",
	})
	input <- NewEntry(Fields{
		"uri": "/zxc/vbn",
		"foo": "456",
		"bar": "567",
		"baz": "678",
	})
	close(input)
	output := make(chan *Entry, 1) // Make it buffered to avoid deadlock
	reducer.Reduce(input, output)

	result, ok := <-output
	assert.True(t, ok)
	value, err := result.FloatField("foo")
	assert.NoError(t, err)
	assert.Equal(t, value, 123.0+456)
	value, err = result.FloatField("bar")
	assert.NoError(t, err)
	assert.Equal(t, value, 234.0+567.0)
	_, err = result.Field("buz")
	assert.Error(t, err)
}

func TestAvgReducer(t *testing.T) {
	reducer := &Avg{[]string{"foo", "bar"}}
	assert.Implements(t, (*Reducer)(nil), reducer)

	// Prepare import channel
	input := make(chan *Entry, 2)
	input <- NewEntry(Fields{
		"uri": "/asd/fgh",
		"foo": "123",
		"bar": "234",
		"baz": "345",
	})
	input <- NewEntry(Fields{
		"uri": "/zxc/vbn",
		"foo": "456",
		"bar": "567",
		"baz": "678",
	})
	close(input)
	output := make(chan *Entry, 1) // Make it buffered to avoid deadlock
	reducer.Reduce(input, output)

	result, ok := <-output
	assert.True(t, ok)
	value, err := result.FloatField("foo")
	assert.NoError(t, err)
	assert.Equal(t, value, (123.0+456)/2.0)
	value, err = result.FloatField("bar")
	assert.NoError(t, err)
	assert.Equal(t, value, (234.0+567.0)/2.0)
	_, err = result.Field("buz")
	assert.Error(t, err)
}

func TestChainReducer(t *testing.T) {
	reducer := NewChain(&Avg{[]string{"foo", "bar"}}, &Count{})
	assert.Implements(t, (*Reducer)(nil), reducer)

	// Prepare import channel
	input := make(chan *Entry, 2)
	input <- NewEntry(Fields{
		"uri": "/asd/fgh",
		"foo": "123",
		"bar": "234",
		"baz": "345",
	})
	input <- NewEntry(Fields{
		"uri": "/zxc/vbn",
		"foo": "456",
		"bar": "567",
		"baz": "678",
	})
	close(input)
	output := make(chan *Entry, 1) // Make it buffered to avoid deadlock
	reducer.Reduce(input, output)

	result, ok := <-output
	assert.True(t, ok)

	value, err := result.FloatField("foo")
	assert.NoError(t, err)
	assert.Equal(t, value, (123.0+456)/2.0)

	value, err = result.FloatField("bar")
	assert.NoError(t, err)
	assert.Equal(t, value, (234.0+567.0)/2.0)

	count, err := result.Field("count")
	assert.NoError(t, err)
	assert.Equal(t, count, "2")

	_, err = result.Field("buz")
	assert.Error(t, err)
}

func TestGroupByReducer(t *testing.T) {
	reducer := NewGroupBy(
		// Fields to group by
		[]string{"host"},
		// Result reducers
		&Sum{[]string{"foo", "bar"}},
		new(Count),
	)
	assert.Implements(t, (*Reducer)(nil), reducer)

	// Prepare import channel
	input := make(chan *Entry, 10)
	input <- NewEntry(Fields{
		"uri":  "/asd/fgh",
		"host": "alpha.example.com",
		"foo":  "1",
		"bar":  "2",
		"baz":  "3",
	})
	input <- NewEntry(Fields{
		"uri":  "/zxc/vbn",
		"host": "beta.example.com",
		"foo":  "4",
		"bar":  "5",
		"baz":  "6",
	})
	input <- NewEntry(Fields{
		"uri":  "/ijk/lmn",
		"host": "beta.example.com",
		"foo":  "7",
		"bar":  "8",
		"baz":  "9",
	})
	close(input)
	output := make(chan *Entry, 2) // Make it buffered to avoid deadlock
	reducer.Reduce(input, output)

	// Collect result entries from output channel to the map, because reading
	// from channel can be in any order, it depends on each reducer processing
	resultMap := make(map[string]*Entry)
	for result := range output {
		value, err := result.Field("host")
		assert.NoError(t, err)
		resultMap[value] = result
	}
	assert.Equal(t, len(resultMap), 2)

	// Read and assert first group result
	result := resultMap["alpha.example.com"]

	floatVal, err := result.FloatField("foo")
	assert.NoError(t, err)
	assert.Equal(t, floatVal, 1.0)

	floatVal, err = result.FloatField("bar")
	assert.NoError(t, err)
	assert.Equal(t, floatVal, 2.0)

	value, err := result.Field("count")
	assert.NoError(t, err)
	assert.Equal(t, value, "1")

	// Read and assert second group result
	result = resultMap["beta.example.com"]

	floatVal, err = result.FloatField("foo")
	assert.NoError(t, err)
	assert.Equal(t, floatVal, 4.0+7.0)

	floatVal, err = result.FloatField("bar")
	assert.NoError(t, err)
	assert.Equal(t, floatVal, 5.0+8.0)

	value, err = result.Field("count")
	assert.NoError(t, err)
	assert.Equal(t, value, "2")
}

func TestIntervalReducer(t *testing.T) {
	reducer := &Interval{
		Field:  "timestamp",
		Format: time.RFC3339,
		Start:  time.Date(2015, time.February, 2, 2, 2, 2, 0, time.UTC),
		End:    time.Date(2015, time.May, 5, 5, 5, 5, 0, time.UTC),
	}
	assert.Implements(t, (*Reducer)(nil), reducer)

	// Prepare import channel
	input := make(chan *Entry, 5)
	input <- NewEntry(Fields{
		"timestamp": "2015-01-01T01:01:01Z",
		"foo":       "123",
		"bar":       "234",
		"baz":       "345",
	})
	input <- NewEntry(Fields{
		"timestamp": "2015-02-02T02:02:02Z",
		"foo":       "456",
		"bar":       "567",
		"baz":       "678",
	})
	input <- NewEntry(Fields{
		"timestamp": "2015-03-03T03:03:03Z",
		"foo":       "789",
		"bar":       "891",
		"baz":       "912",
	})
	input <- NewEntry(Fields{
		"timestamp": "2015-04-04T04:04:04Z",
		"foo":       "123",
		"bar":       "234",
		"baz":       "345",
	})
	input <- NewEntry(Fields{
		"timestamp": "2015-05-05T05:05:05Z",
		"foo":       "456",
		"bar":       "567",
		"baz":       "678",
	})
	close(input)

	output := make(chan *Entry, 5) // Make it buffered to avoid deadlock
	reducer.Reduce(input, output)

	want := []string{
		"'timestamp'=2015-02-02T02:02:02Z;'foo'=456;'bar'=567;'baz'=678",
		"'timestamp'=2015-03-03T03:03:03Z;'foo'=789;'bar'=891;'baz'=912",
		"'timestamp'=2015-04-04T04:04:04Z;'foo'=123;'bar'=234;'baz'=345",
	}
	results := []string{}

	for {
		result, ok := <-output
		if !ok {
			break
		}
		results = append(
			results,
			result.FieldsHash([]string{
				"timestamp",
				"foo",
				"bar",
				"baz",
			}),
		)
	}
	assert.Equal(t, results, want)
}
