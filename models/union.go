package models

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// marshalVariant encodes the populated variant of a tagged union such as
// RichBlock, InputRichBlock or RichText.
//
// setTag stamps the discriminator on a copy of the variant rather than on the
// value the caller handed us, so encoding has no side effects and the same value
// can be encoded from several goroutines at once. A variant that was never set is
// reported as an error instead of panicking inside encoding/json.
func marshalVariant[T any, K ~string](union string, tag K, variant *T, setTag func(*T)) ([]byte, error) {
	if variant == nil {
		return nil, fmt.Errorf("nil variant for %s type %q", union, tag)
	}

	v := *variant
	setTag(&v)

	return json.Marshal(&v)
}

// marshalUnknownVariant encodes a tagged union whose discriminator this library does
// not know, so a value decoded from a newer Bot API release still round trips. Only
// the discriminator is emitted: no variant was populated, there is nothing else to
// write. field is the JSON name of the discriminator ("type", "status", "source").
//
// An empty tag is an unset value rather than a future variant and stays an error.
func marshalUnknownVariant[K ~string](union, field string, tag K) ([]byte, error) {
	if tag == "" {
		return nil, fmt.Errorf("unsupported %s type %q", union, tag)
	}

	return json.Marshal(map[string]K{field: tag})
}

// missingDiscriminator reports a tagged object that carries no discriminator at all.
// That is a malformed value rather than a variant from a future release: there is no
// tag to keep, and MarshalJSON would reject the empty Type on the way back out.
func missingDiscriminator(union string) error {
	return fmt.Errorf("%s object without a discriminator", union)
}

func archivedVariantField(data []byte, field, tag string) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	return archivedVariant(fields[field], tag)
}

// archivedVariant recognizes the nested Go struct emitted by older versions
// without confusing fields such as a transaction's Telegram User with a variant.
func archivedVariant(nested []byte, tag string) ([]byte, error) {
	if len(nested) == 0 || nested[0] != '{' {
		return nil, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(nested, &fields); err != nil {
		return nil, err
	}
	typ := fields["type"]
	if typ == nil {
		typ = fields["Type"]
	}
	switch string(typ) {
	case strconv.Quote(tag):
		return nested, nil
	case `""`:
		// The old encoder wrote a blank nested type for caller-constructed variants.
		delete(fields, "Type")
		fields["type"] = json.RawMessage(strconv.Quote(tag))
		return json.Marshal(fields)
	}
	return nil, nil
}
