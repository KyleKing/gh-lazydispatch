package logs

import (
	"testing"
)

func BenchmarkParseLogOutput_SmallLog(b *testing.B) {
	// Load small fixture (24-58 lines)
	logContent := LoadFixture(b, "successful_run.txt")

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		ParseLogOutput(logContent, "test-step")
	}
}

func BenchmarkParseLogOutput_1kLines(b *testing.B) {
	logContent := GenerateLargeLogFixture(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		ParseLogOutput(logContent, "test-step")
	}
}

func BenchmarkParseLogOutput_10kLines(b *testing.B) {
	logContent := GenerateLargeLogFixture(10000)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		ParseLogOutput(logContent, "test-step")
	}
}

func BenchmarkParseLogOutput_50kLines(b *testing.B) {
	logContent := GenerateLargeLogFixture(50000)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		ParseLogOutput(logContent, "test-step")
	}
}

func BenchmarkParseLogOutput_WithErrors(b *testing.B) {
	// 10k lines with 10% errors
	logContent := GenerateLargeLogWithErrors(10000, 0.1)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		ParseLogOutput(logContent, "test-step")
	}
}

func BenchmarkParseLogOutput_Unicode(b *testing.B) {
	logContent := GenerateUnicodeLog()

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		ParseLogOutput(logContent, "test-step")
	}
}

func BenchmarkParseLogOutput_ANSI(b *testing.B) {
	logContent := GenerateANSILog()

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		ParseLogOutput(logContent, "test-step")
	}
}

func BenchmarkParseLogOutput_MixedContent(b *testing.B) {
	logContent := GenerateMixedLog(5000)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		ParseLogOutput(logContent, "test-step")
	}
}

func BenchmarkParseLogOutput_WithTimestamps(b *testing.B) {
	logContent := GenerateLogWithTimestamps(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		ParseLogOutput(logContent, "test-step")
	}
}

func BenchmarkParseLogOutput_MultiStep(b *testing.B) {
	// 10 steps with 100 lines each = 1000 lines total
	logContent := GenerateMultiStepLog(10, 100)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		ParseLogOutput(logContent, "test-step")
	}
}
