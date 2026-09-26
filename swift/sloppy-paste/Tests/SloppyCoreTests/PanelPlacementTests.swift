import Foundation
import CoreGraphics
import Testing
@testable import SloppyCore

@Suite struct PanelPlacementTests {
    // A 1440×900 built-in display as main, a 2560×1440 display to its right
    // (bottom-aligned), and a 1920×1080 display above the main one.
    let main = PanelPlacement.Screen(
        frame: CGRect(x: 0, y: 0, width: 1440, height: 900),
        visibleFrame: CGRect(x: 0, y: 0, width: 1440, height: 875))
    let right = PanelPlacement.Screen(
        frame: CGRect(x: 1440, y: 0, width: 2560, height: 1440),
        visibleFrame: CGRect(x: 1440, y: 70, width: 2560, height: 1345))
    let above = PanelPlacement.Screen(
        frame: CGRect(x: -240, y: 900, width: 1920, height: 1080),
        visibleFrame: CGRect(x: -240, y: 900, width: 1920, height: 1055))
    var screens: [PanelPlacement.Screen] { [main, right, above] }

    @Test func picksTheScreenWithTheMouse() {
        #expect(PanelPlacement.screen(containing: CGPoint(x: 100, y: 100), in: screens) == main)
        #expect(PanelPlacement.screen(containing: CGPoint(x: 3000, y: 1200), in: screens) == right)
        #expect(PanelPlacement.screen(containing: CGPoint(x: -200, y: 1500), in: screens) == above)
    }

    @Test func sharedEdgesFollowNSMouseInRect() {
        // x == 1440 is the right edge of main and the left edge of right.
        #expect(PanelPlacement.screen(containing: CGPoint(x: 1440, y: 10), in: screens) == right)
        // y == 900 is the top edge of main and the bottom edge of above.
        #expect(PanelPlacement.screen(containing: CGPoint(x: 10, y: 900), in: screens) == main)
    }

    @Test func pointOutsideEveryScreenPicksTheNearest() {
        #expect(PanelPlacement.screen(containing: CGPoint(x: 4100, y: 500), in: screens) == right)
        #expect(PanelPlacement.screen(containing: CGPoint(x: 500, y: -50), in: screens) == main)
        #expect(PanelPlacement.screen(containing: .zero, in: []) == nil)
    }

    @Test func centresOnTheVisibleFrame() {
        let origin = PanelPlacement.origin(
            for: CGSize(width: 760, height: 480), in: right.visibleFrame, verticalFraction: 0.6)
        #expect(origin == CGPoint(x: 1440 + 1280 - 380, y: 70 + (1345 - 480) * 0.6))
    }

    @Test func shrinksAndClampsOnASmallScreen() {
        let small = CGRect(x: 100, y: 50, width: 700, height: 400)
        let size = PanelPlacement.fittedSize(CGSize(width: 760, height: 480), in: small)
        #expect(size == CGSize(width: 660, height: 360))
        let origin = PanelPlacement.origin(for: size, in: small, verticalFraction: 0.6)
        #expect(origin == CGPoint(x: 120, y: 50 + 40 * 0.6))
    }

    @Test func oversizedPanelStartsAtTheVisibleOrigin() {
        let visible = CGRect(x: 0, y: 0, width: 300, height: 200)
        let origin = PanelPlacement.origin(for: CGSize(width: 400, height: 300), in: visible, verticalFraction: 0.5)
        #expect(origin == CGPoint(x: 0, y: 0))
    }

    @Test func frameUsesTheMouseScreen() throws {
        let frame = try #require(PanelPlacement.frame(
            preferred: CGSize(width: 760, height: 480), mouse: CGPoint(x: -100, y: 1000),
            screens: screens, verticalFraction: 0.6))
        #expect(above.visibleFrame.contains(frame))
        #expect(frame.midX == above.visibleFrame.midX)
    }
}
