// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

package rtmr

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-eventlog/register"
	"github.com/google/go-tdx-guest/abi"
	tdxpb "github.com/google/go-tdx-guest/proto/tdx"
	"github.com/google/go-tdx-guest/testing/testdata"
)

func TestReplayCcelWithTdQuote(t *testing.T) {
	ccelBytes, err := os.ReadFile("../testing/testdata/ccel/ccel_data.dat")
	if err != nil {
		t.Fatal(err)
	}
	tableBytes, err := os.ReadFile("../testing/testdata/ccel/ccel_table.dat")
	if err != nil {
		t.Fatal(err)
	}
	quoteBytes, err := os.ReadFile("../testing/testdata/ccel/cos-113-tdx-quote.dat")
	if err != nil {
		t.Fatal(err)
	}
	nonceBytes, err := os.ReadFile("../testing/testdata/ccel/nonce.dat")
	if err != nil {
		t.Fatal(err)
	}
	option := TdxDefaultOpts(nonceBytes)
	quote, err := abi.QuoteToProto(quoteBytes)
	if err != nil {
		t.Fatal(err)
	}
	ccel, err := ParseCcelWithTdQuote(ccelBytes, tableBytes, quote, &option)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ccel)
}

func TestGetRtmrsFromTdQuoteSuccess(t *testing.T) {
	rtmr0 := bytes.Repeat([]byte{0x11}, abi.RtmrSize)
	rtmr1 := bytes.Repeat([]byte{0x22}, abi.RtmrSize)
	rtmr2 := bytes.Repeat([]byte{0x33}, abi.RtmrSize)
	rtmr3 := bytes.Repeat([]byte{0x44}, abi.RtmrSize)
	syntheticRtmrs := [][]byte{rtmr0, rtmr1, rtmr2, rtmr3}
	wantSyntheticBank := &register.RTMRBank{
		RTMRs: []register.RTMR{
			{Index: 0, Digest: rtmr0},
			{Index: 1, Digest: rtmr1},
			{Index: 2, Digest: rtmr2},
			{Index: 3, Digest: rtmr3},
		},
	}

	rawQuoteV4, err := abi.QuoteToProto(testdata.RawQuote)
	if err != nil {
		t.Fatalf("abi.QuoteToProto(RawQuote) unexpected error: %v", err)
	}
	quoteV4, ok := rawQuoteV4.(*tdxpb.QuoteV4)
	if !ok {
		t.Fatalf("abi.QuoteToProto(RawQuote) type = %T, want *tdxpb.QuoteV4", rawQuoteV4)
	}

	rawQuoteV5, err := abi.QuoteToProto(testdata.RawQuoteV5)
	if err != nil {
		t.Fatalf("abi.QuoteToProto(RawQuoteV5) unexpected error: %v", err)
	}
	quoteV5, ok := rawQuoteV5.(*tdxpb.QuoteV5)
	if !ok {
		t.Fatalf("abi.QuoteToProto(RawQuoteV5) type = %T, want *tdxpb.QuoteV5", rawQuoteV5)
	}
	v5BodyRtmrs := quoteV5.GetTdQuoteBodyDescriptor().GetTdQuoteBodyV5().GetRtmrs()

	tests := []struct {
		name  string
		quote any
		want  *register.RTMRBank
	}{
		{
			name: "quote_v4_synthetic",
			quote: &tdxpb.QuoteV4{
				TdQuoteBody: &tdxpb.TDQuoteBody{Rtmrs: syntheticRtmrs},
			},
			want: wantSyntheticBank,
		},
		{
			name: "quote_v5_synthetic",
			quote: &tdxpb.QuoteV5{
				TdQuoteBodyDescriptor: &tdxpb.TDQuoteBodyDescriptor{
					TdQuoteBodyV5: &tdxpb.TDQuoteBodyV5{Rtmrs: syntheticRtmrs},
				},
			},
			want: wantSyntheticBank,
		},
		{
			name:  "quote_v4_raw_sample",
			quote: quoteV4,
			want: &register.RTMRBank{
				RTMRs: []register.RTMR{
					{Index: 0, Digest: quoteV4.GetTdQuoteBody().GetRtmrs()[0]},
					{Index: 1, Digest: quoteV4.GetTdQuoteBody().GetRtmrs()[1]},
					{Index: 2, Digest: quoteV4.GetTdQuoteBody().GetRtmrs()[2]},
					{Index: 3, Digest: quoteV4.GetTdQuoteBody().GetRtmrs()[3]},
				},
			},
		},
		{
			name:  "quote_v5_raw_sample",
			quote: quoteV5,
			want: &register.RTMRBank{
				RTMRs: []register.RTMR{
					{Index: 0, Digest: v5BodyRtmrs[0]},
					{Index: 1, Digest: v5BodyRtmrs[1]},
					{Index: 2, Digest: v5BodyRtmrs[2]},
					{Index: 3, Digest: v5BodyRtmrs[3]},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GetRtmrsFromTdQuote(tc.quote)
			if err != nil {
				t.Fatalf("GetRtmrsFromTdQuote() unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("GetRtmrsFromTdQuote() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetRtmrsFromTdQuoteErrors(t *testing.T) {
	fiveRtmrs := [][]byte{{1}, {2}, {3}, {4}, {5}}

	tests := []struct {
		name    string
		quote   any
		wantErr string
	}{
		{
			name:    "nil_quote",
			quote:   nil,
			wantErr: "unsupported quote type",
		},
		{
			name:    "unsupported_quote_type",
			quote:   "invalid",
			wantErr: "unsupported quote type",
		},
		{
			name:    "typed_nil_quote_v4",
			quote:   (*tdxpb.QuoteV4)(nil),
			wantErr: abi.ErrQuoteV4Nil.Error(),
		},
		{
			name:    "quote_v4_nil_body",
			quote:   &tdxpb.QuoteV4{},
			wantErr: abi.ErrTDQuoteBodyNil.Error(),
		},
		{
			name: "quote_v4_too_many_rtmrs",
			quote: &tdxpb.QuoteV4{
				TdQuoteBody: &tdxpb.TDQuoteBody{Rtmrs: fiveRtmrs},
			},
			wantErr: "too many RTMRs in quote",
		},
		{
			name:    "typed_nil_quote_v5",
			quote:   (*tdxpb.QuoteV5)(nil),
			wantErr: abi.ErrQuoteV5Nil.Error(),
		},
		{
			name:    "quote_v5_nil_descriptor",
			quote:   &tdxpb.QuoteV5{},
			wantErr: abi.ErrTDQuoteBodyDescriptorNil.Error(),
		},
		{
			name: "quote_v5_nil_body",
			quote: &tdxpb.QuoteV5{
				TdQuoteBodyDescriptor: &tdxpb.TDQuoteBodyDescriptor{},
			},
			wantErr: abi.ErrTDQuoteBodyNil.Error(),
		},
		{
			name: "quote_v5_too_many_rtmrs",
			quote: &tdxpb.QuoteV5{
				TdQuoteBodyDescriptor: &tdxpb.TDQuoteBodyDescriptor{
					TdQuoteBodyV5: &tdxpb.TDQuoteBodyV5{Rtmrs: fiveRtmrs},
				},
			},
			wantErr: "too many RTMRs in quote",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GetRtmrsFromTdQuote(tc.quote)
			if err == nil {
				t.Fatalf("GetRtmrsFromTdQuote() = %v, want error containing %q", got, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("GetRtmrsFromTdQuote() error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}
