import Foundation
import CoreGraphics
import Testing
@testable import SloppyCore

@Suite struct PanelSizeTests {
    let visible = CGRect(x: 0, y: 0, width: 1440, height: 875)
    let start = CGRect(x: 340, y: 200, width: 760, height: 480)

    private func drag(_ edges: PanelSize.Edges, _ dx: CGFloat, _ dy: CGFloat) -> CGRect {
        PanelSize.dragged(
            start, edges: edges, by: CGVector(dx: dx, dy: dy),
            minimum: PanelSize.minimum, maximum: PanelSize.maximum, in: visible)
    }

    @Test func sanitizesStoredSizes() {
        #expect(PanelSize.sanitized(width: nil, height: nil) == PanelSize.defaultSize)
        #expect(PanelSize.sanitized(width: .nan, height: -1) == PanelSize.defaultSize)
        #expect(PanelSize.sanitized(width: 900.4, height: 500) == CGSize(width: 900, height: 500))
        #expect(PanelSize.sanitized(width: 100, height: 99_999) == CGSize(width: 600, height: 2000))
    }

    @Test func eitherSideGrowsBothSides() {
        #expect(drag(.right, 50, 0) == CGRect(x: 290, y: 200, width: 860, height: 480))
        #expect(drag(.left, -50, 0) == CGRect(x: 290, y: 200, width: 860, height: 480))
    }

    @Test func eitherEdgeGrowsBothEdgesVertically() {
        #expect(drag(.top, 0, 40) == CGRect(x: 340, y: 160, width: 760, height: 560))
        #expect(drag(.bottom, 0, -40) == CGRect(x: 340, y: 160, width: 760, height: 560))
    }

    @Test func draggingInwardShrinks() {
        #expect(drag(.right, -30, 0) == CGRect(x: 370, y: 200, width: 700, height: 480))
        #expect(drag(.top, 25, -30) == CGRect(x: 340, y: 230, width: 760, height: 420))
    }

    @Test func cornersResizeBothDimensions() {
        #expect(drag([.right, .top], 20, 10) == CGRect(x: 320, y: 190, width: 800, height: 500))
    }

    @Test func stopsAtTheMinimum() {
        #expect(drag(.right, -500, 0).size == CGSize(width: 600, height: 480))
    }

    @Test func staysOnScreen() {
        let frame = drag(.right, 1000, 0)
        #expect(frame == CGRect(x: 20, y: 200, width: 1400, height: 480))
    }
}
