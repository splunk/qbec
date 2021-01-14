local makeFooBar = function (foo, bar) {
    foo: foo,
    bar: bar,
};

{
    makeFooBar:: makeFooBar,
}
