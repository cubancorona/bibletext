package android.widget;
public class EditText extends android.view.View {
    public int setTextCalls,focusCalls;
    public boolean composing;
    private Value value = new Value("");
    private int start,end;
    private final java.util.List<android.text.TextWatcher> watchers = new java.util.ArrayList<android.text.TextWatcher>();
    public EditText(android.content.Context context) {}
    public void setText(String text) {
        setTextCalls++; composing=false; value=new Value(text); start=end=text.length();
        for(android.text.TextWatcher watcher:watchers) watcher.afterTextChanged(value);
    }
    public android.text.Editable getText() { return value; }
    public void setSelection(int offset) { start=end=offset; }
    public void setSelection(int first,int last) { start=first; end=last; }
    public int getSelectionStart() { return start; }
    public int getSelectionEnd() { return end; }
    public boolean requestFocus() { focusCalls++; return true; }
    public void addTextChangedListener(android.text.TextWatcher watcher) { watchers.add(watcher); }
    public android.graphics.Paint getPaint() { return new android.graphics.Paint(); }
    public void setIncludeFontPadding(boolean value) {}
    public void setGravity(int value) {}
    public void setSingleLine(boolean value) {}
    public void setInputType(int value) {}
    public void setImeOptions(int value) {}
    public void setImportantForAutofill(int value) {}
    public void setImportantForContentCapture(int value) {}
    public void setHorizontallyScrolling(boolean value) {}
    public void setTextSize(int unit,float value) {}
    public void setTextColor(int value) {}
    public void setHintTextColor(int value) {}
    public void setHint(String value) {}
    public void setPadding(int left,int top,int right,int bottom) {}
    private static class Value implements android.text.Editable {
        private final String text;
        Value(String value) { text=value; }
        public int length() { return text.length(); }
        public char charAt(int index) { return text.charAt(index); }
        public CharSequence subSequence(int first,int last) { return text.subSequence(first,last); }
        public String toString() { return text; }
    }
}
