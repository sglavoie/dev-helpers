import Foundation
import CoreGraphics
import Testing
@testable import SloppyCore

@Suite struct DetailPaneWidthTests {
    @Test func sanitizesStoredWidths() {
        #expect(DetailPaneWidth.sanitized(nil) == 330)
        #expect(DetailPaneWidth.sanitized(.infinity) == 330)
        #expect(DetailPaneWidth.sanitized(0) == 330)
        #expect(DetailPaneWidth.sanitized(400.6) == 401)
        #expect(DetailPaneWidth.sanitized(10) == 240)
        #expect(DetailPaneWidth.sanitized(9_000) == 2000)
    }

    @Test func leavesRoomForTheList() {
        #expect(DetailPaneWidth.displayed(330, in: 760) == 330)
        #expect(DetailPaneWidth.displayed(600, in: 760) == 480)
        #expect(DetailPaneWidth.displayed(330, in: 400) == 240)
    }

    @Test func draggingLeftWidensThePane() {
        #expect(DetailPaneWidth.dragged(from: 330, by: -50, in: 760) == 380)
        #expect(DetailPaneWidth.dragged(from: 330, by: 50, in: 760) == 280)
    }

    @Test func dragStopsAtTheLimits() {
        #expect(DetailPaneWidth.dragged(from: 330, by: 500, in: 760) == 240)
        #expect(DetailPaneWidth.dragged(from: 330, by: -500, in: 760) == 480)
    }
}
