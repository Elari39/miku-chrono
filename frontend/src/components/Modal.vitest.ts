import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import DurationText from "./DurationText.vue";
import Modal from "./Modal.vue";

describe("DurationText", () => {
  it("renders the human form by default", () => {
    const wrapper = mount(DurationText, { props: { seconds: 3661 } });
    expect(wrapper.text()).toBe("1时01分");
  });

  it("renders the clock form with mono styling in clock mode", () => {
    const wrapper = mount(DurationText, { props: { seconds: 3661, mode: "clock" } });
    expect(wrapper.text()).toBe("1:01:01");
    expect(wrapper.classes()).toContain("font-mono");
  });

  it("applies mono styling on request in human mode", () => {
    const wrapper = mount(DurationText, { props: { seconds: 45, mono: true } });
    expect(wrapper.text()).toBe("45秒");
    expect(wrapper.classes()).toContain("font-mono");
  });
});

function mountModal(
  props: { open: boolean; title: string; wide?: boolean },
  slots: Record<string, string> = {},
) {
  // Stub Teleport so dialog content stays inside the wrapper's tree, and
  // attach to the document so the topmost-dialog logic sees real DOM.
  return mount(Modal, {
    props,
    slots,
    attachTo: document.body,
    global: { stubs: { Teleport: true } },
  });
}

describe("Modal", () => {
  it("renders nothing while closed", () => {
    const wrapper = mountModal({ open: false, title: "标题" });
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false);
    wrapper.unmount();
  });

  it("renders the title and default slot when open", () => {
    const wrapper = mountModal({ open: true, title: "补录记录" }, { default: "<p>表单内容</p>" });
    const dialog = wrapper.find('[role="dialog"]');
    expect(dialog.exists()).toBe(true);
    expect(dialog.attributes("aria-modal")).toBe("true");
    expect(dialog.text()).toContain("补录记录");
    expect(dialog.text()).toContain("表单内容");
    wrapper.unmount();
  });

  it("emits close when the backdrop is clicked", async () => {
    const wrapper = mountModal({ open: true, title: "标题" });
    await wrapper.get(".fixed").trigger("click");
    expect(wrapper.emitted("close")).toHaveLength(1);
    wrapper.unmount();
  });

  it("emits close on Escape", () => {
    const wrapper = mountModal({ open: true, title: "标题" });
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(wrapper.emitted("close")).toHaveLength(1);
    wrapper.unmount();
  });
});
