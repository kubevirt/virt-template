package generator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExpressionValue", func() {
	Describe("GenerateValue", func() {
		var g ExpressionValue

		BeforeEach(func() {
			g = ExpressionValue{}
		})

		It("should generate lowercase letters", func() {
			val, err := g.GenerateValue("[a-z]{8}")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(8))
			Expect(val).To(MatchRegexp("^[a-z]{8}$"))
		})

		It("should generate uppercase letters", func() {
			val, err := g.GenerateValue("[A-Z]{5}")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(5))
			Expect(val).To(MatchRegexp("^[A-Z]{5}$"))
		})

		It("should generate digits", func() {
			val, err := g.GenerateValue("[0-9]{10}")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(10))
			Expect(val).To(MatchRegexp("^[0-9]{10}$"))
		})

		It("should generate alphanumeric characters", func() {
			val, err := g.GenerateValue("[a-zA-Z0-9]{16}")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(16))
			Expect(val).To(MatchRegexp("^[a-zA-Z0-9]{16}$"))
		})

		It("should generate with prefix and suffix", func() {
			val, err := g.GenerateValue("test[0-9]{1}x")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(6))
			Expect(val).To(MatchRegexp("^test[0-9]x$"))
		})

		It("should generate hexadecimal values", func() {
			val, err := g.GenerateValue("0x[A-F0-9]{4}")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(6))
			Expect(val).To(MatchRegexp("^0x[A-F0-9]{4}$"))
		})

		It("should handle multiple pattern replacements", func() {
			val, err := g.GenerateValue("[a-z]{2}-[0-9]{3}")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(MatchRegexp("^[a-z]{2}-[0-9]{3}$"))
		})

		It(`should generate with \w pattern (word characters)`, func() {
			val, err := g.GenerateValue(`[\w]{10}`)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(10))
			Expect(val).To(MatchRegexp(`^[\w]{10}$`))
		})

		It(`should generate with \d pattern (digits)`, func() {
			val, err := g.GenerateValue(`[\d]{8}`)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(8))
			Expect(val).To(MatchRegexp(`^[\d]{8}$`))
		})

		It(`should generate with \a pattern (alphanumeric)`, func() {
			val, err := g.GenerateValue(`[\a]{12}`)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(12))
			Expect(val).To(MatchRegexp("^[a-zA-Z0-9]{12}$"))
		})

		It(`should generate with \A pattern (symbols)`, func() {
			val, err := g.GenerateValue(`[\A]{5}`)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(5))
			Expect(val).To(MatchRegexp(`^[!"#$%&'\(\)*\+,-./:;<=>\?@\[\\\]^_` + "`" + `\{\|\}~]{5}$`))
		})

		It("should handle mixed patterns", func() {
			val, err := g.GenerateValue(`[a-zA-Z\d]{10}`)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(10))
			Expect(val).To(MatchRegexp("^[a-zA-Z0-9]{10}$"))
		})

		It("should handle specific character sets", func() {
			val, err := g.GenerateValue("[abc123]{6}")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(6))
			Expect(val).To(MatchRegexp("^[abc123]{6}$"))
		})

		It("should handle minimum length of 1", func() {
			val, err := g.GenerateValue("[a-z]{1}")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(1))
			Expect(val).To(MatchRegexp("^[a-z]$"))
		})

		It("should handle maximum length of 255", func() {
			val, err := g.GenerateValue("[a-z]{255}")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(HaveLen(255))
			Expect(val).To(MatchRegexp("^[a-z]+$"))
		})

		It("should return error for invalid length", func() {
			val, err := g.GenerateValue("[a-z]{abc}")
			Expect(err).To(MatchError(ContainSubstring("malformed length syntax")))
			Expect(val).To(BeEmpty())
		})

		It("should return error for length below minimum", func() {
			val, err := g.GenerateValue("[a-z]{0}")
			Expect(err).To(MatchError(ContainSubstring("range must be within")))
			Expect(val).To(BeEmpty())
		})

		It("should return error for length above maximum", func() {
			val, err := g.GenerateValue("[a-z]{256}")
			Expect(err).To(MatchError(ContainSubstring("range must be within")))
			Expect(val).To(BeEmpty())
		})

		Describe("Edge cases", func() {
			It("should handle expression at start of string", func() {
				val, err := g.GenerateValue("[a-z]{3}suffix")
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(HaveLen(9))
				Expect(val).To(MatchRegexp("^[a-z]{3}suffix$"))
			})

			It("should handle expression at end of string", func() {
				val, err := g.GenerateValue("prefix[a-z]{3}")
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(HaveLen(9))
				Expect(val).To(MatchRegexp("^prefix[a-z]{3}$"))
			})

			It("should handle expression in middle of string", func() {
				val, err := g.GenerateValue("pre[a-z]{3}post")
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(HaveLen(10))
				Expect(val).To(MatchRegexp("^pre[a-z]{3}post$"))
			})

			It("should handle single character alphabet", func() {
				val, err := g.GenerateValue("[a]{10}")
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal("aaaaaaaaaa"))
			})

			It("should generate different values on multiple calls", func() {
				const size = 100
				vals := make(map[string]bool)
				for range size {
					val, err := g.GenerateValue("[a-z]{20}")
					Expect(err).ToNot(HaveOccurred())
					vals[val] = true
				}
				Expect(vals).To(HaveLen(size))
			})

			It("should handle input that does not contain a valid expression", func() {
				const in = "[invalid"
				val, err := g.GenerateValue(in)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal(in))
			})
		})
	})

	Describe("extractRangesAndLength", func() {
		It("should extract simple range and length", func() {
			r, l, err := extractRangesAndLength("[a-z]{8}")
			Expect(err).ToNot(HaveOccurred())
			Expect(r).To(Equal("a-z"))
			Expect(l).To(Equal(8))
		})

		It("should extract complex range and length", func() {
			r, l, err := extractRangesAndLength("[a-zA-Z0-9]{16}")
			Expect(err).ToNot(HaveOccurred())
			Expect(r).To(Equal("a-zA-Z0-9"))
			Expect(l).To(Equal(16))
		})

		It("should return error for malformed ranges", func() {
			r, l, err := extractRangesAndLength("[]{8}")
			Expect(err).To(MatchError("malformed ranges syntax: "))
			Expect(r).To(BeEmpty())
			Expect(l).To(BeZero())
		})

		It("should return error for malformed length", func() {
			r, l, err := extractRangesAndLength("[a-z]{abc}")
			Expect(err).To(MatchError("malformed length syntax: strconv.Atoi: parsing \"abc\": invalid syntax"))
			Expect(r).To(BeEmpty())
			Expect(l).To(BeZero())
		})

		It("should return error for length too small", func() {
			r, l, err := extractRangesAndLength("[a-z]{0}")
			Expect(err).To(MatchError("range must be within [1-255] characters: 0"))
			Expect(r).To(BeEmpty())
			Expect(l).To(BeZero())
		})

		It("should return error for length too large", func() {
			r, l, err := extractRangesAndLength("[a-z]{300}")
			Expect(err).To(MatchError(ContainSubstring("range must be within [1-255] characters: 300")))
			Expect(r).To(BeEmpty())
			Expect(l).To(BeZero())
		})
	})

	Describe("getAlphabetFromRanges", func() {
		It("should generate alphabet from lowercase range", func() {
			a, err := getAlphabetFromRanges("a-z")
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("abcdefghijklmnopqrstuvwxyz"))
		})

		It("should generate alphabet from uppercase range", func() {
			a, err := getAlphabetFromRanges("A-Z")
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("ABCDEFGHIJKLMNOPQRSTUVWXYZ"))
		})

		It("should generate alphabet from numeric range", func() {
			a, err := getAlphabetFromRanges("0-9")
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("0123456789"))
		})

		It("should combine multiple ranges", func() {
			a, err := getAlphabetFromRanges("a-zA-Z")
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"))
		})

		It(`should handle \w pattern`, func() {
			a, err := getAlphabetFromRanges(`\w`)
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz"))
		})

		It(`should handle \d pattern`, func() {
			a, err := getAlphabetFromRanges(`\d`)
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("0123456789"))
		})

		It(`should handle \a pattern`, func() {
			a, err := getAlphabetFromRanges(`\a`)
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"))
		})

		It(`should handle \A pattern (symbols)`, func() {
			a, err := getAlphabetFromRanges(`\A`)
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"))
		})

		It("should handle literal characters", func() {
			a, err := getAlphabetFromRanges("abc123")
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("123abc"))
		})

		It("should remove duplicates", func() {
			a, err := getAlphabetFromRanges("aabbcc")
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("abc"))
		})

		It("should return error for empty ranges", func() {
			_, err := getAlphabetFromRanges("")
			Expect(err).To(MatchError("malformed ranges syntax: "))
		})
	})

	Describe("subAlphabet", func() {
		It("should create alphabet from lowercase range", func() {
			a, err := subAlphabet('a', 'z')
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("abcdefghijklmnopqrstuvwxyz"))
		})

		It("should create alphabet from uppercase range", func() {
			a, err := subAlphabet('A', 'Z')
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("ABCDEFGHIJKLMNOPQRSTUVWXYZ"))
		})

		It("should create alphabet from numeric range", func() {
			a, err := subAlphabet('0', '9')
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("0123456789"))
		})

		It("should create partial alphabet from lowercase range", func() {
			a, err := subAlphabet('a', 'e')
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("abcde"))
		})

		It("should create partial alphabet from uppercase range", func() {
			a, err := subAlphabet('A', 'E')
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("ABCDE"))
		})

		It("should create partial alphabet from numeric range", func() {
			a, err := subAlphabet('0', '7')
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal("01234567"))
		})

		DescribeTable("should create alphabet from mixed range", func(from, to, expected string) {
			a, err := subAlphabet(from[0], to[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(a).To(Equal(expected))
		},
			Entry("a to 7", "a", "7", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567"),
			Entry("a to 7", "a", "Z", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"),
			Entry("a to 7", "j", "9", "jklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"),
			Entry("a to 7", "l", "W", "lmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVW"),
			Entry("a to 7", "G", "2", "GHIJKLMNOPQRSTUVWXYZ012"),
		)

		It("should return error for invalid start character", func() {
			a, err := subAlphabet('!', 'z')
			Expect(err).To(MatchError("invalid start character in range: !"))
			Expect(a).To(BeEmpty())
		})

		It("should return error for invalid end character", func() {
			a, err := subAlphabet('a', '!')
			Expect(err).To(MatchError("invalid end character in range: !"))
			Expect(a).To(BeEmpty())
		})

		It("should return error for reversed lowercase range", func() {
			a, err := subAlphabet('z', 'a')
			Expect(err).To(MatchError("invalid range specified: z-a"))
			Expect(a).To(BeEmpty())
		})

		It("should return error for reversed uppercase range", func() {
			a, err := subAlphabet('Z', 'A')
			Expect(err).To(MatchError("invalid range specified: Z-A"))
			Expect(a).To(BeEmpty())
		})

		It("should return error for reversed numeric range", func() {
			a, err := subAlphabet('9', '0')
			Expect(err).To(MatchError("invalid range specified: 9-0"))
			Expect(a).To(BeEmpty())
		})

		It("should return error for reversed mixed range", func() {
			a, err := subAlphabet('Z', 'a')
			Expect(err).To(MatchError("invalid range specified: Z-a"))
			Expect(a).To(BeEmpty())
		})
	})

	Describe("removeDuplicates", func() {
		It("should remove duplicate characters", func() {
			a := removeDuplicates("aabbcc")
			Expect(a).To(Equal("abc"))
		})

		It("should handle string without duplicates", func() {
			const in = "abc"
			a := removeDuplicates(in)
			Expect(a).To(Equal(in))
		})

		It("should handle empty string", func() {
			a := removeDuplicates("")
			Expect(a).To(BeEmpty())
		})

		It("should sort and remove duplicates", func() {
			a := removeDuplicates("cbacba")
			Expect(a).To(Equal("abc"))
		})
	})

	Describe("replaceWithGeneratedValue", func() {
		It("should replace expression with generated value", func() {
			result, err := replaceWithGeneratedValue("test[a-z]{5}end", "[a-z]{5}", "abcde", 5)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(12))
			Expect(result).To(MatchRegexp("^test[a-z]{5}end$"))
		})

		It("should generate value of correct length", func() {
			result, err := replaceWithGeneratedValue("[0-9]{10}", "[0-9]{10}", "0123456789", 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(10))
		})

		It("should return error for empty alphabet", func() {
			val, err := replaceWithGeneratedValue("test", "test", "", 5)
			Expect(err).To(MatchError("alphabet cannot be empty: test"))
			Expect(val).To(BeEmpty())
		})

		It("should replace only first occurrence", func() {
			val, err := replaceWithGeneratedValue("[a-z]{1}[a-z]{1}", "[a-z]{1}", "a", 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal("a[a-z]{1}"))
		})

		It("should not replace when expression does not match", func() {
			const in = "noexpression"
			val, err := replaceWithGeneratedValue(in, "[a-z]{10}", "abcdef", 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal(in))
		})
	})
})
