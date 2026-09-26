import Foundation
import CoreGraphics
import Testing
@testable import SloppyCore

@Suite struct PanelZoomTests {
    let visible = CGRect(x: 0, y: 0, width: 1440, height: 875)

    @Test func stepsThroughTheLevels() {
        #expect(PanelZoom.zoomedIn(from: 1) == 1.1)
        #expect(PanelZoom.zoomedOut(from: 1) == 0.85)
        #expect(PanelZoom.zoomedIn(from: 1.1) == 1.25)
    }

    @Test func stopsAtTheEnds() {
        #expect(PanelZoom.zoomedIn(from: 2) == 2)
        #expect(PanelZoom.zoomedOut(from: 0.75) == 0.75)
    }

    @Test func offLevelValuesStepToTheNeighbouringLevel() {
        #expect(PanelZoom.zoomedIn(from: 1.2) == 1.25)
        #expect(PanelZoom.zoomedOut(from: 1.2) == 1.1)
    }

    @Test func sanitizesStoredValues() {
        #expect(PanelZoom.sanitized(nil) == 1)
        #expect(PanelZoom.sanitized(.nan) == 1)
        #expect(PanelZoom.sanitized(-3) == 1)
        #expect(PanelZoom.sanitized(1.26) == 1.25)
        #expect(PanelZoom.sanitized(40) == 2)
    }

    @Test func resizesAroundTheCentre() {
        let frame = CGRect(x: 340, y: 200, width: 760, height: 480)
        let resized = PanelZoom.resized(frame, to: CGSize(width: 950, height: 600), in: visible)
        #expect(resized == CGRect(x: 245, y: 140, width: 950, height: 600))
    }

    @Test func staysOnScreenWhenGrowingNearAnEdge() {
        let frame = CGRect(x: 30, y: 370, width: 760, height: 480)
        let resized = PanelZoom.resized(frame, to: CGSize(width: 950, height: 600), in: visible)
        #expect(resized == CGRect(x: 20, y: 255, width: 950, height: 600))
    }

    @Test func shrinksToFitTheScreen() {
        let frame = CGRect(x: 340, y: 200, width: 760, height: 480)
        let resized = PanelZoom.resized(frame, to: CGSize(width: 1520, height: 960), in: visible)
        #expect(resized == CGRect(x: 20, y: 20, width: 1400, height: 835))
    }
}
