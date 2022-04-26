package base

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"app/base/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const DateFormat = "2006-01-02"
const (
	SearchQuery           = "search"
	PublishedQuery        = "published"
	SeverityQuery         = "severity"
	CvssScoreQuery        = "cvss_score"
	AffectedClustersQuery = "affected_clusters"
	AffectedImagesQuery   = "affected_images"
	LimitQuery            = "limit"
	OffsetQuery           = "offset"
	SortQuery             = "sort"
)

const (
	SortFilterArgs = "sort_filter"
)

const (
	CveSearch             string = "CveSearch"
	ExposedClustersSearch string = "ExposedClustersSearch"
)

// Filter interface, represents filter obtained from
// query argument in request link.
type Filter interface {
	ApplyQuery(*gorm.DB, map[string]interface{}) error
	RawQueryVal() string
	RawQueryVals() []string
	RawQueryName() string
}

// RawFilter implements Filter interface, contains
// raw name of query argument and raw parsed query
// values in string
type RawFilter struct {
	RawParam  string
	RawValues []string
}

// RawQueryName getter to the parameter name in query
func (b *RawFilter) RawQueryName() string {
	return b.RawParam
}

// RawQueryVal returns obtained raw values formatted in query value string
func (b *RawFilter) RawQueryVal() string {
	return strings.Join(b.RawValues[:], ",")
}

// RawQueryVals returns parsed raw values from query value string
func (b *RawFilter) RawQueryVals() []string {
	return b.RawValues
}

// Search represents filter for CVE substring search
// ex. search=CVE-2022
type Search struct {
	RawFilter
	value string
}

// ApplyQuery filters CVEs by their substring match name or description
func (c *Search) ApplyQuery(tx *gorm.DB, args map[string]interface{}) error {
	regex := fmt.Sprintf("%%%s%%", c.value)

	switch args[SearchQuery] {
	case ExposedClustersSearch:
		tx.Where("cve.name LIKE ? OR cve.description LIKE ?", regex, regex)
		return nil
	case CveSearch:
		tx.Where("cluster.uuid LIKE ?", regex)
		return nil
	}
	return nil
}

// CvePublishDate represents filter for CVE publish date filtering
// ex: publsihed=2021-01-01,2022-02-02
type CvePublishDate struct {
	RawFilter
	From time.Time
	To   time.Time
}

// ApplyQuery filters CVEs by their public date limit
func (c *CvePublishDate) ApplyQuery(tx *gorm.DB, _ map[string]interface{}) error {
	tx.Where("cve.public_date >= ? AND cve.public_date <= ?", c.From, c.To)
	return nil
}

// Severity represents CVE severity filter
// ex. severity=critical,important,none
type Severity struct {
	RawFilter
	Value []models.Severity
}

// ApplyQuery filters CVEs by their severity
func (s *Severity) ApplyQuery(tx *gorm.DB, _ map[string]interface{}) error {
	tx.Where("cve.severity IN ?", s.Value)
	return nil
}

// CvssScore represents filter for CVE cvss2/3 score range
// cvss_score=0.0,9.0
type CvssScore struct {
	RawFilter
	From float32
	To   float32
}

// ApplyQuery filters CVEs by cvss2/3 score range
func (c *CvssScore) ApplyQuery(tx *gorm.DB, _ map[string]interface{}) error {
	tx.Where("COALESCE(cve.cvss3_score, cve.cvss2_score) >= ? AND COALESCE(cve.cvss3_score, cve.cvss2_score) <= ?", c.From, c.To)
	return nil
}

// To be implemented
type AffectingClusters struct {
	RawFilter
	OneOrMore bool
	None      bool
}

func (a *AffectingClusters) ApplyQuery(tx *gorm.DB, _ map[string]interface{}) error {
	return nil
}

// To be implemented
type AffectingImages struct {
	RawFilter
	OneOrMore bool
	None      bool
}

func (a *AffectingImages) ApplyQuery(tx *gorm.DB, _ map[string]interface{}) error {
	return nil
}

// Limit filter sets number of data objects per page
// ex. limit=20
type Limit struct {
	RawFilter
	Value uint64
}

// ApplyQuery limits the number of data in query - limit per page
func (l *Limit) ApplyQuery(tx *gorm.DB, _ map[string]interface{}) error {
	tx.Limit(int(l.Value))
	return nil
}

// Offset filter sets an offset of data in query - start of the page
// ex. offset=40
type Offset struct {
	RawFilter
	Value uint64
}

// ApplyQuery sets and offset from the rows result
func (o *Offset) ApplyQuery(tx *gorm.DB, _ map[string]interface{}) error {
	tx.Offset(int(o.Value))
	return nil
}

// SortItem represents an single column row sort expression
// Used by the Sort filter
type SortItem struct {
	Column string
	Desc   bool
}

// SortArgs represents an argument for Sort filter
// SortableColumns represents mapping from user selected column
// to the correct sql expression column
// DefaultSortable contains a default sorting defined by controller
type SortArgs struct {
	SortableColumns map[string]string
	DefaultSortable []SortItem
}

// Sort filter sorts a query by given list of sort item expressions
// ex. sort=synopsis,cvss_score
type Sort struct {
	RawFilter
	Values []SortItem
}

// ApplyQuery sorts the resulting query, query is sorted
// 1st - by user defined columns
// 2nd - by controller selected default columns
func (s *Sort) ApplyQuery(tx *gorm.DB, args map[string]interface{}) error {
	if i, exists := args[SortFilterArgs]; exists {
		sortArgs, ok := i.(SortArgs)
		if !ok {
			return nil
		}
		// Sort by user selected columns
		for _, item := range s.Values {
			// Check if selected user column is mappable to sortable column sql expression
			if col, exists := sortArgs.SortableColumns[item.Column]; exists {
				if item.Desc {
					tx.Order(fmt.Sprintf("%s DESC NULLS LAST", col))
				} else {
					tx.Order(fmt.Sprintf("%s ASC NULLS LAST", col))
				}
			} else {
				return errors.New("invalid sort column selected")
			}
		}
		// Sort by default sortable
		for _, item := range sortArgs.DefaultSortable {
			if col, exists := sortArgs.SortableColumns[item.Column]; exists {
				// Always add the default sort parameter, so user can see default sort
				if item.Desc {
					tx.Order(fmt.Sprintf("%s DESC NULLS LAST", col))
					s.RawValues = append(s.RawValues, fmt.Sprintf("-%s", item.Column))
				} else {
					tx.Order(fmt.Sprintf("%s ASC NULLS LAST", col))
					s.RawValues = append(s.RawValues, item.Column)
				}
			}
		}
	}
	return nil
}

// GetRequestedFilters gets requested parsed filters from gin context
// returns empty map if not exists
func GetRequestedFilters(ctx *gin.Context) map[string]Filter {
	if f, exists := ctx.Get("filters"); exists {
		if f, ok := f.(map[string]Filter); ok {
			return f
		}
	}
	return map[string]Filter{}
}

// ApplyFilters applies requested filters from query params on created query from controller,
// filters needs to be allowed from controller in allowedFilters array
func ApplyFilters(query *gorm.DB, allowedFilters []string, requestedFilters map[string]Filter, args map[string]interface{}) error {
	for _, allowedFilter := range allowedFilters {
		if filter, requested := requestedFilters[allowedFilter]; requested {
			err := filter.ApplyQuery(query, args)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
