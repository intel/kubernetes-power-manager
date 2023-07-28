package cpuset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Utilities

// Help Function for if slices equal
func areSlicesEqual(sliceA, sliceB []int) bool {

	if sliceA == nil && sliceB == nil {
		return true
	}

	if sliceA == nil || sliceB == nil {
		return false
	}

	if len(sliceA) != len(sliceB) {
		return false
	}

	freqA := make(map[int]int)
	freqB := make(map[int]int)

	for _, elem := range sliceA {
		freqA[elem]++
	}
	for _, elem := range sliceB {
		freqB[elem]++
	}

	for elem, countA := range freqA {
		countB, exists := freqB[elem]
		if !exists || countA != countB {
			return false
		}
	}
	return true
}

// Help Function for if slices are equal int 64
func areSlicesEqualInt64(sliceA, sliceB []int64) bool {

	if sliceA == nil && sliceB == nil {
		return true
	}

	if sliceA == nil || sliceB == nil {
		return false
	}

	if len(sliceA) != len(sliceB) {
		return false
	}

	setA := make(map[int64]struct{})
	for _, elem := range sliceA {
		setA[elem] = struct{}{}
	}

	for _, elem := range sliceB {
		if _, found := setA[elem]; !found {
			return false
		}
	}

	return true
}

//Tests

func TestNewBuilder(t *testing.T) {
	builder := NewBuilder()

	//Assert that the elemennt map is initialised ant not nil
	if builder.result.elems == nil {
		t.Errorf("Expected elems map to be initialized, but it was nil")
	}
}

func TestAdd(t *testing.T) {
	// Test case -- Adding elements to empty CPUSet
	builder := NewBuilder()
	builder.Add(1, 2, 3)

	assert.True(t, areSlicesEqualInt64(builder.result.ToSliceInt64(), []int64{1, 2, 3}))

	// Test case -- Adding elements to non-empty CPUSet
	builderB := NewBuilder()
	builderB.Add(1)
	builderB.Add(2, 3)

	assert.True(t, areSlicesEqualInt64(builderB.result.ToSliceInt64(), []int64{1, 2, 3}))

	// Test case -- Adding elements after calling Result (shouldn't have an effect)
	builderC := NewBuilder()
	builderC.Add(1, 2)
	builderC.Result()

	builderC.Add(3, 4)

	assert.True(t, areSlicesEqualInt64(builderC.result.ToSliceInt64(), []int64{1, 2}))
}

func TestResult(t *testing.T) {
	//Test case -- Test with elements added
	builderA := NewBuilder()
	builderA.Add(1, 2, 3)

	cpuSetA := builderA.Result()

	expectedA := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
		},
	}

	assert.True(t, expectedA.Equals(cpuSetA))
	//Test case -- Test builder is marked "done"
	builderB := NewBuilder()
	builderB.Result()

	if !builderB.done {
		t.Error("Expected: Builder to be marked 'done'. Actual: Builder not marked 'done'.")
	}

	//Test case -- Checks Adds dont effect result
	builderC := NewBuilder()
	builderC.Result()
	builderC.Add(4)

	if _, found := builderC.result.elems[4]; found {
		t.Errorf("Expected: Element 4 not added into the CPUSet. Actual: Element 4 was added")
	}
}

func TestNewCPUSet(t *testing.T) {
	//Test case -- Create a CPUSet with elements
	cpusetA := NewCPUSet(1, 2, 3)

	//Checks elemnts are present
	for i := 1; i <= 3; i++ {
		if _, found := cpusetA.elems[i]; !found {
			t.Errorf("Expected: Element %d in CPUSet. Actual: Element was not found", i)
		}
	}

	//Test case -- Create a CPUSet with single element
	cpusetB := NewCPUSet(10)

	if _, found := cpusetB.elems[10]; !found {
		t.Errorf("Expected: Element 10 in CPUSet. Actual: Element wasn't found")
	}

	cpusetC := NewCPUSet()

	if len(cpusetC.elems) != 0 {
		t.Errorf("Expected: An empty CPUSet. Actual: CpuSet had %d elements", len(cpusetC.elems))
	}
}

func TestSize(t *testing.T) {
	//Test case -- Size of empty set
	cpusetA := &CPUSet{
		elems: map[int]struct{}{},
	}

	sizeA := cpusetA.Size()
	assert.Equal(t, 0, sizeA)

	//Test case -- Size of a non-empty set
	cpusetB := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
		},
	}

	sizeB := cpusetB.Size()
	assert.Equal(t, 3, sizeB)
}

func TestIsEmpty(t *testing.T) {
	//Test case -- Empty CPUSet
	cpusetA := &CPUSet{
		elems: map[int]struct{}{},
	}

	assert.True(t, cpusetA.IsEmpty())

	//Test case -- Non-empty set
	cpusetB := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
		},
	}

	assert.False(t, cpusetB.IsEmpty())
}

func TestContains(t *testing.T) {
	//Test case -- CPUSet contains the element
	cpusetA := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
		},
	}

	if !cpusetA.Contains(1) {
		t.Errorf("Expected: Set contains element 1. Actual: Element 1 not found in set")
	}

	//Test case -- Non-empty set
	cpusetB := &CPUSet{
		elems: map[int]struct{}{
			4: {},
			5: {},
			6: {},
		},
	}

	if cpusetB.Contains(1) {
		t.Errorf("Expected: Set doesn't contains element 1. Actual: Element 1 found in set")
	}
}

func TestEquals(t *testing.T) {
	//Test case -- Two equal sets
	cpusetA := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
		},
	}

	cpusetB := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
		},
	}

	assert.True(t, cpusetA.Equals(cpusetB))

	//Test case -- Two unequals
	cpusetX := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
		},
	}

	cpusetY := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
		},
	}

	assert.False(t, cpusetX.Equals(cpusetY))
}

func TestFilter(t *testing.T) {
	//Test case: Filter even numbers
	cpusetA := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
			4: {},
			5: {},
		},
	}

	isEven := func(n int) bool {
		return n%2 == 0
	}

	filteredA := cpusetA.Filter(isEven)

	if _, found := filteredA.elems[1]; found {
		t.Error(("Expected: odd number 1 filtered out. Actual: 1 still in set"))
	}

	if _, found := filteredA.elems[2]; !found {
		t.Error(("Expected: even number 2 found. Actual: 2 has been filtered out"))
	}

	//Test case: Filter numbers smaller than 4
	cpusetB := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
			4: {},
			5: {},
		},
	}

	isGreaterThan3 := func(n int) bool {
		return n > 3
	}

	filteredB := cpusetB.Filter(isGreaterThan3)

	if _, found := filteredB.elems[1]; found {
		t.Error(("Expected: low number 1 filtered out. Actual: 1 still in set"))
	}

	if _, found := filteredB.elems[3]; found {
		t.Error(("Expected: low number 3 filtered out. Actual: 3 still in set"))
	}

	if _, found := filteredB.elems[4]; !found {
		t.Error(("Expected: high number 4 not filtered out. Actual: 4 filtered from the set"))
	}
}

func TestFilterNot(t *testing.T) {
	//Test case: Filter even numbers
	cpusetA := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
			4: {},
			5: {},
		},
	}

	isEven := func(n int) bool {
		return n%2 == 0
	}

	filteredA := cpusetA.FilterNot(isEven)

	if _, found := filteredA.elems[1]; !found {
		t.Error(("Expected: odd number 1 not filtered out. Actual: 1 filtered out"))
	}

	if _, found := filteredA.elems[2]; found {
		t.Error(("Expected: even number 2 filtered out. Actual: 2 has not been filtered out"))
	}

	//Test case: Filter numbers smaller than 4
	cpusetB := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
			4: {},
			5: {},
		},
	}

	isLessThanOrEqualTo3 := func(n int) bool {
		return n <= 3
	}

	filteredB := cpusetB.FilterNot(isLessThanOrEqualTo3)

	if _, found := filteredB.elems[1]; found {
		t.Error(("Expected: low number 1 filtered out. Actual: 1 still in set"))
	}

	if _, found := filteredB.elems[3]; found {
		t.Error(("Expected: low number 3 filtered out. Actual: 3 still in set"))
	}

	if _, found := filteredB.elems[4]; !found {
		t.Error(("Expected: high number 4 not filtered out. Actual: 4 filtered from the set"))
	}
}

func TestSubsetOf(t *testing.T) {
	//Test case -- Empty set is a subset of any set
	cpusetA := &CPUSet{
		elems: map[int]struct{}{},
	}
	cpusetB := &CPUSet{
		elems: map[int]struct{}{},
	}

	if !cpusetA.IsSubsetOf(*cpusetB) {
		t.Error("Expected: Empty set to be a subset of any set. Actual: It was not")
	}

	//Test case -- Proper subset
	cpusetJ := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
		},
	}
	cpusetK := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
		},
	}

	if !cpusetJ.IsSubsetOf(*cpusetK) {
		t.Error("Expected: SetJ is a subset of setK. Actual: It was not a subset")
	}

	//Test case -- Empty set is a subset of any set
	cpusetX := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
		},
	}
	cpusetY := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			3: {},
		},
	}

	if cpusetX.IsSubsetOf(*cpusetY) {
		t.Error("Expected: SetX is not a subset of setY. Actual: It was a subset")
	}
}

func TestUnion(t *testing.T) {

	//Test case -- Two empty
	cpusetA := CPUSet{
		elems: map[int]struct{}{},
	}
	cpusetB := CPUSet{
		elems: map[int]struct{}{},
	}

	expectedA := CPUSet{
		elems: map[int]struct{}{},
	}

	unionA := cpusetA.Union(cpusetB)
	assert.True(t, expectedA.Equals(unionA))

	//Test case -- One empty, one with two elements
	cpusetX := CPUSet{
		elems: map[int]struct{}{},
	}
	cpusetY := CPUSet{
		elems: map[int]struct{}{
			1: {},
			3: {},
		},
	}

	expectedB := CPUSet{
		elems: map[int]struct{}{
			1: {},
			3: {},
		},
	}

	unionB := cpusetX.Union(cpusetY)
	assert.True(t, expectedB.Equals(unionB))
}

func TestUnionAll(t *testing.T) {
	//Test case -- Empty set union with empty slices should result in an empty set
	cpusetA := CPUSet{
		elems: map[int]struct{}{},
	}

	expectedA := CPUSet{
		elems: map[int]struct{}{},
	}

	var emptySets []CPUSet
	union_allA := cpusetA.UnionAll(emptySets)

	assert.True(t, expectedA.Equals(union_allA))

	//Test case -- Non-empty set union with non-empty sets
	cpusetJ := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
		},
	}
	cpusetK := CPUSet{
		elems: map[int]struct{}{
			2: {},
			3: {},
		},
	}
	cpusetL := CPUSet{
		elems: map[int]struct{}{
			3: {},
			4: {},
		},
	}

	expectedB := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
			4: {},
		},
	}

	sets := []CPUSet{cpusetK, cpusetL}
	union_allB := cpusetJ.UnionAll(sets)

	assert.True(t, expectedB.Equals(union_allB))
}

func TestIntersection(t *testing.T) {
	// Test case -- Empty set intersection with another empty set should result in an empty set
	cpusetA := CPUSet{
		elems: map[int]struct{}{},
	}
	cpusetB := CPUSet{
		elems: map[int]struct{}{},
	}
	expectedA := CPUSet{
		elems: map[int]struct{}{},
	}

	intersectionA := cpusetA.Intersection(cpusetB)
	assert.True(t, expectedA.Equals(intersectionA))

	//Test case -- Non empty set intersects with an empty should produce empty
	cpusetJ := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
		},
	}
	cpusetK := CPUSet{
		elems: map[int]struct{}{},
	}

	expectedB := CPUSet{
		elems: map[int]struct{}{},
	}

	intersectionB := cpusetJ.Intersection(cpusetK)
	assert.True(t, expectedB.Equals(intersectionB))

	//Test case -- Non-empty set intersects with another non-empty set
	cpusetX := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
		},
	}
	cpusetY := CPUSet{
		elems: map[int]struct{}{
			2: {},
			3: {},
			4: {},
		},
	}

	expectedC := CPUSet{
		elems: map[int]struct{}{
			2: {},
			3: {},
		},
	}

	intersectionC := cpusetX.Intersection(cpusetY)
	assert.True(t, expectedC.Equals(intersectionC))
}

func TestDifference(t *testing.T) {
	//Test case -- Empty set intersection with another empty set should result in an empty set
	cpusetA := CPUSet{
		elems: map[int]struct{}{},
	}
	cpusetB := CPUSet{
		elems: map[int]struct{}{},
	}
	expectedA := CPUSet{
		elems: map[int]struct{}{},
	}

	differenceA := cpusetA.Difference(cpusetB)
	assert.True(t, expectedA.Equals(differenceA))
	//Test case -- Non empty set difference with an empty should produce non-empty
	cpusetJ := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
		},
	}
	cpusetK := CPUSet{
		elems: map[int]struct{}{},
	}
	expectedB := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
		},
	}

	differenceB := cpusetJ.Difference(cpusetK)
	assert.True(t, expectedB.Equals(differenceB))
}

func TestToSlice(t *testing.T) {
	//Test case -- Empty set should return empty slice
	cpusetA := &CPUSet{
		elems: map[int]struct{}{},
	}
	sliceA := cpusetA.ToSlice()
	if len(sliceA) != 0 {
		t.Error("Expected: Empty slice. Actual: Non-empty slice")
	}

	//Test case -- Non-empty should sort and return slice
	cpusetB := &CPUSet{
		elems: map[int]struct{}{
			4: {},
			3: {},
			2: {},
			1: {},
		},
	}

	expectedSlice := []int{1, 2, 3, 4}

	sliceB := cpusetB.ToSlice()

	assert.Equal(t, expectedSlice, sliceB)
}

func TestToSliceNoSort(t *testing.T) {
	//Test case -- Empty set should return empty slice
	cpusetA := &CPUSet{
		elems: map[int]struct{}{},
	}
	sliceA := cpusetA.ToSliceNoSort()
	if len(sliceA) != 0 {
		t.Error("Expected: Empty slice. Actual: Non-empty slice")
	}

	//Test case -- Non-empty should sort and return slice
	cpusetB := &CPUSet{
		elems: map[int]struct{}{
			4: {},
			3: {},
			2: {},
			1: {},
		},
	}

	expectedSlice := []int{4, 3, 2, 1}

	sliceB := cpusetB.ToSliceNoSort()
	if !areSlicesEqual(sliceB, expectedSlice) {
		t.Errorf("Expected: slice %v. Actual: slice%v", expectedSlice, sliceB)
	}
}

func TestToSliceInt64(t *testing.T) {
	//Test case -- Empty set should return empty slice
	cpusetA := &CPUSet{
		elems: map[int]struct{}{},
	}
	sliceA := cpusetA.ToSliceInt64()
	assert.Nil(t, sliceA)

	//Test case -- Non-empty should sort and return slice
	cpusetB := &CPUSet{
		elems: map[int]struct{}{
			4: {},
			3: {},
			2: {},
			1: {},
		},
	}

	expectedSlice := []int64{1, 2, 3, 4}

	sliceB := cpusetB.ToSliceInt64()
	assert.Equal(t, expectedSlice, sliceB)
}

func TestToSliceNoSortInt64(t *testing.T) {

	//Test case -- Empty set should return empty slice
	cpusetA := &CPUSet{
		elems: map[int]struct{}{},
	}

	var expectedSliceA []int64

	sliceA := cpusetA.ToSliceNoSortInt64()
	assert.True(t, areSlicesEqualInt64(sliceA, expectedSliceA))

	//Test case -- Non-empty should sort and return slice
	cpusetB := &CPUSet{
		elems: map[int]struct{}{
			4: {},
			3: {},
			2: {},
			1: {},
		},
	}

	expectedSliceB := []int64{4, 2, 3, 1}

	sliceB := cpusetB.ToSliceNoSortInt64()
	assert.True(t, areSlicesEqualInt64(sliceB, expectedSliceB))
}

func TestString(t *testing.T) {
	//Test case -- Empty set should return an empty string
	cpusetA := &CPUSet{
		elems: map[int]struct{}{},
	}

	expectedStringA := ""

	resultA := cpusetA.String()
	assert.Equal(t, expectedStringA, resultA)

	//Test case -- Non-empty set with consecutive elements should return string with ranges
	cpusetB := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
			4: {},
			6: {},
			8: {},
			9: {},
		},
	}

	expectedStringB := "1-4,6,8-9"
	resultB := cpusetB.String()
	assert.Equal(t, expectedStringB, resultB)

	//Test case -- Non-empty set with non-consecutive elements should return string with individual elements
	cpusetC := &CPUSet{
		elems: map[int]struct{}{
			1: {},
			3: {},
			5: {},
			7: {},
		},
	}

	expectedStringC := "1,3,5,7"
	resultC := cpusetC.String()
	assert.Equal(t, expectedStringC, resultC)
}

func TestMustParse(t *testing.T) {
	//Test case -- Valid string parsed
	inputStrA := "1,2,3,5-7,10"
	expectedA := CPUSet{
		elems: map[int]struct{}{
			1:  {},
			2:  {},
			3:  {},
			5:  {},
			6:  {},
			7:  {},
			10: {},
		},
	}

	resultA := MustParse(inputStrA)
	assert.Equal(t, expectedA, resultA)
}

func TestParse(t *testing.T) {
	//Test case -- Valid string parsed
	inputStrA := ""
	expectedA := CPUSet{
		elems: map[int]struct{}{},
	}

	resultA, errA := Parse(inputStrA)
	assert.NoError(t, errA)
	assert.Equal(t, expectedA, resultA)

	//Test case -- Single elements seperated by commas
	inputStrB := "1,3,5,7"
	expectedB := CPUSet{
		elems: map[int]struct{}{
			1: {},
			3: {},
			5: {},
			7: {},
		},
	}

	resultB, errB := Parse(inputStrB)
	assert.NoError(t, errB)
	assert.Equal(t, expectedB, resultB)

	//Test case -- Range of elemenst
	inputStrC := "1-4,6,8-9"
	expectedC := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
			3: {},
			4: {},
			6: {},
			8: {},
			9: {},
		},
	}

	resultC, errC := Parse(inputStrC)
	assert.NoError(t, errC)
	assert.Equal(t, expectedC, resultC)
}

func TestClone(t *testing.T) {
	//Test case -- Empty set should return empty
	cpusetA := CPUSet{
		elems: map[int]struct{}{},
	}

	cloneA := cpusetA.Clone()
	assert.True(t, cpusetA.Equals(cloneA))

	//Test case -- Non-Empty set should return mom-empty
	cpusetB := CPUSet{
		elems: map[int]struct{}{
			1: {},
			2: {},
		},
	}

	cloneB := cpusetB.Clone()
	assert.True(t, cpusetB.Equals(cloneB))
}
